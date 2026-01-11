import * as vscode from 'vscode';
import * as fs from 'fs';
import * as path from 'path';
import * as crypto from 'crypto';

export interface SessionMetadata {
    id: string;
    title: string;
    lastModified: number;
    messageCount: number;
    workspaceDir: string;
}

export interface SessionData {
    messages: any[]; // ChatMessage[]
    todos: any[];    // Todo[]
}

export class SessionService {
    private readonly globalStateKey = 'ricochet_sessions';
    private readonly storageDir: string;

    constructor(
        private readonly context: vscode.ExtensionContext
    ) {
        this.storageDir = path.join(context.globalStorageUri.fsPath, 'sessions');
        if (!fs.existsSync(this.storageDir)) {
            fs.mkdirSync(this.storageDir, { recursive: true });
        }
    }

    public async listSessions(): Promise<SessionMetadata[]> {
        const sessions = this.context.globalState.get<SessionMetadata[]>(this.globalStateKey, []);
        // Sort by lastModified desc
        return sessions.sort((a, b) => b.lastModified - a.lastModified);
    }

    public async createSession(workspaceDir: string, title?: string): Promise<string> {
        const id = crypto.randomUUID();
        const metadata: SessionMetadata = {
            id,
            title: title || 'New Chat',
            lastModified: Date.now(),
            messageCount: 0,
            workspaceDir
        };

        // Update list
        const sessions = await this.listSessions();
        sessions.unshift(metadata);
        await this.context.globalState.update(this.globalStateKey, sessions);

        // Create empty session file
        const data: SessionData = { messages: [], todos: [] };
        const filePath = path.join(this.storageDir, `${id}.json`);
        fs.writeFileSync(filePath, JSON.stringify(data, null, 2));

        return id;
    }

    public async loadSession(id: string): Promise<SessionData | null> {
        const filePath = path.join(this.storageDir, `${id}.json`);
        if (fs.existsSync(filePath)) {
            try {
                const content = fs.readFileSync(filePath, 'utf-8');
                return JSON.parse(content);
            } catch (e) {
                console.error(`Failed to load session ${id}:`, e);
            }
        }
        return null;
    }

    public async saveSession(id: string, data: SessionData, workspaceDir: string): Promise<void> {
        const filePath = path.join(this.storageDir, `${id}.json`);
        fs.writeFileSync(filePath, JSON.stringify(data, null, 2));

        // Update metadata
        const sessions = await this.listSessions();
        const index = sessions.findIndex(s => s.id === id);

        let title = "New Chat";
        if (data.messages.length > 0) {
            // Simple title generation from first user message
            const firstUserMsg = data.messages.find(m => m.role === 'user');
            if (firstUserMsg) {
                title = firstUserMsg.content.slice(0, 50) + (firstUserMsg.content.length > 50 ? '...' : '');
            }
        }

        const metadata: SessionMetadata = {
            id,
            title,
            lastModified: Date.now(),
            messageCount: data.messages.length,
            workspaceDir
        };

        if (index !== -1) {
            sessions[index] = metadata;
        } else {
            sessions.unshift(metadata);
        }
        await this.context.globalState.update(this.globalStateKey, sessions);
    }

    public async deleteSession(id: string): Promise<void> {
        const filePath = path.join(this.storageDir, `${id}.json`);
        if (fs.existsSync(filePath)) {
            fs.unlinkSync(filePath);
        }

        const sessions = await this.listSessions();
        const updated = sessions.filter(s => s.id !== id);
        await this.context.globalState.update(this.globalStateKey, updated);
    }

    public async appendMessage(sessionId: string, message: any): Promise<void> {
        const sessionData = await this.loadSession(sessionId);
        if (!sessionData) {
            console.warn(`[SessionService] Cannot append to non-existent session: ${sessionId}`);
            return;
        }

        sessionData.messages.push(message);

        // Use the workspace from metadata if possible, or fallback
        const sessions = await this.listSessions();
        const sessionMeta = sessions.find(s => s.id === sessionId);
        const workspaceDir = sessionMeta ? sessionMeta.workspaceDir : ''; // Ideally we shouldn't lose this

        await this.saveSession(sessionId, sessionData, workspaceDir);
    }
}
