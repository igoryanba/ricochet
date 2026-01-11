import * as vscode from 'vscode';
import { spawn, ChildProcess } from 'child_process';
import * as path from 'path';
import * as readline from 'readline';
import * as fs from 'fs';

export interface CoreMessage {
    type: string;
    payload: unknown;
}

/**
 * Manages the ricochet-core Go process lifecycle.
 * Communicates via JSON-RPC over stdio.
 */
export class CoreProcess {
    private process: ChildProcess | null = null;
    private messageHandlers: Map<string, (payload: unknown) => void> = new Map();
    private requestHandlers: Map<string, (payload: unknown) => Promise<unknown>> = new Map();
    private pendingRequests: Map<number, { resolve: (value: unknown) => void; reject: (error: Error) => void }> = new Map();
    private requestId = 0;
    private rl: readline.Interface | null = null;

    constructor(private rootPath: string, private extensionPath: string) { }

    async start(): Promise<void> {
        const binaryPath = this.getBinaryPath();

        // Load default API keys from .env.keys file
        const envKeys = this.loadEnvKeys();

        console.log(`[Extension] Starting core process: ${binaryPath} in ${this.rootPath}`);

        this.process = spawn(binaryPath, ['--stdio'], {
            cwd: this.rootPath,
            stdio: ['pipe', 'pipe', 'pipe'],
            env: { ...process.env, ...envKeys, PROJECT_ROOT: this.rootPath }
        });

        if (!this.process.stdout || !this.process.stdin) {
            throw new Error('Failed to start core process: stdio not available');
        }

        // Setup readline for JSON-RPC messages
        this.rl = readline.createInterface({
            input: this.process.stdout,
            crlfDelay: Infinity
        });

        this.rl.on('line', (line) => {
            const trimmed = line.trim();
            if (!trimmed || !trimmed.startsWith('{')) {
                // Not a JSON message, probably a log from the core
                if (trimmed) {
                    console.log(`[ricochet-core] LOG: ${trimmed}`);
                }
                return;
            }

            console.log(`[Core -> Ext] RAW: ${line.substring(0, 100)}${line.length > 100 ? '...' : ''}`);
            try {
                const message = JSON.parse(line);
                this.handleMessage(message);
            } catch (error) {
                console.error(`[Core -> Ext] Failed to parse JSON: ${line.substring(0, 100)}`, error);
            }
        });

        this.process.stderr?.on('data', (data) => {
            console.error(`[ricochet-core] ${data}`);
        });

        this.process.on('exit', (code) => {
            console.log(`ricochet-core exited with code ${code}`);
            this.process = null;
        });

        // Wait for ready message
        await this.waitForReady();
    }

    async stop(): Promise<void> {
        if (this.process) {
            this.process.kill('SIGTERM');
            this.process = null;
        }
        if (this.rl) {
            this.rl.close();
            this.rl = null;
        }
    }

    async send(type: string, payload: unknown): Promise<unknown> {
        if (!this.process?.stdin) {
            throw new Error('Core process not running');
        }

        const id = ++this.requestId;
        const message = JSON.stringify({ id, type, payload }) + '\n';
        console.log(`[Ext -> Core] SEND id=${id} type=${type}`);

        return new Promise((resolve, reject) => {
            this.pendingRequests.set(id, { resolve, reject });
            this.process!.stdin!.write(message);

            // Timeout after 5 minutes (increased from 30s for long AI tasks)
            setTimeout(() => {
                if (this.pendingRequests.has(id)) {
                    console.error(`[Ext -> Core] TIMEOUT id=${id} type=${type}`);
                    this.pendingRequests.delete(id);
                    reject(new Error('Request timeout after 5 minutes'));
                }
            }, 300000);
        });
    }

    onMessage(type: string, handler: (payload: unknown) => void): void {
        this.messageHandlers.set(type, handler);
    }

    onRequest(type: string, handler: (payload: unknown) => Promise<unknown>): void {
        this.requestHandlers.set(type, handler);
    }

    private async handleMessage(message: any): Promise<void> {
        console.log(`[Core -> Ext] RECV id=${message.id} type=${message.type}`);
        // Handle response to pending request (Extension -> Core -> Extension)
        if (message.type === 'response' || ('id' in message && this.pendingRequests.has(message.id))) {
            const pending = this.pendingRequests.get(message.id);
            if (pending) {
                console.log(`[Core -> Ext] RESOLVING id=${message.id}`);
                this.pendingRequests.delete(message.id);
                if (message.error) {
                    pending.reject(new Error(String(message.error)));
                } else {
                    pending.resolve(message.payload);
                }
                return;
            } else {
                console.warn(`[Core -> Ext] No pending request for id=${message.id}`);
            }
        }

        // Handle incoming request from core (Core -> Extension -> Core)
        // Correcting protocol check: if it has an ID and is NOT a response, it's a request
        if (message.id && message.type !== 'response') {
            const handler = this.requestHandlers.get(message.type);
            if (handler) {
                try {
                    const result = await handler(message.payload);
                    this.sendResponse(message.id, result);
                } catch (error) {
                    this.sendResponse(message.id, null, String(error));
                }
                return;
            }
        }

        // Handle push notifications
        const handler = this.messageHandlers.get(message.type);
        if (handler) {
            handler(message.payload);
        }
    }

    private sendResponse(id: string | number, payload: unknown, error?: string): void {
        if (!this.process?.stdin) return;
        const message = JSON.stringify({
            id,
            type: 'response',
            payload,
            error
        }) + '\n';
        this.process.stdin.write(message);
    }

    private async waitForReady(): Promise<void> {
        return new Promise((resolve, reject) => {
            const timeout = setTimeout(() => {
                reject(new Error('Core process did not start in time'));
            }, 10000);

            this.onMessage('ready', () => {
                clearTimeout(timeout);
                resolve();
            });
        });
    }

    private getBinaryPath(): string {
        const platform = process.platform;
        const arch = process.arch;

        let binaryName = 'ricochet-core';
        if (platform === 'win32') {
            binaryName += '.exe';
        }

        // Binary is bundled with extension
        return path.join(this.extensionPath, 'bin', `${platform}-${arch}`, binaryName);
    }

    /**
     * Load default API keys from .env.keys file
     */
    private loadEnvKeys(): Record<string, string> {
        const envPath = path.join(this.extensionPath, '.env.keys');
        const result: Record<string, string> = {};

        try {
            if (fs.existsSync(envPath)) {
                const content = fs.readFileSync(envPath, 'utf-8');
                for (const line of content.split('\n')) {
                    const trimmed = line.trim();
                    if (trimmed && !trimmed.startsWith('#')) {
                        const eqIndex = trimmed.indexOf('=');
                        if (eqIndex > 0) {
                            const key = trimmed.substring(0, eqIndex).trim();
                            const value = trimmed.substring(eqIndex + 1).trim();
                            result[key] = value;
                        }
                    }
                }
                console.log(`Loaded ${Object.keys(result).length} API keys from .env.keys`);
            }
        } catch (error) {
            console.warn('Failed to load .env.keys:', error);
        }

        return result;
    }
}
