import * as vscode from 'vscode';
import { CoreProcess } from './core-process';
import { McpServerManager } from './services/mcp/McpServerManager';
import { McpHub } from './services/mcp/McpHub';
import { ShadowCheckpointService } from './services/checkpoints/ShadowCheckpointService';
import { SessionService } from './services/session/SessionService';
import { ChatService } from './services/chat/ChatService';
import { McpService } from './services/mcp/McpService';
import { AgentService } from './services/agent/AgentService';
import * as path from 'path';
import * as fs from 'fs';
import * as nodeCrypto from 'crypto';


/**
 * WebviewProvider for Ricochet sidebar panel.
 * Handles communication between webview UI and core process.
 */
export class WebviewProvider implements vscode.WebviewViewProvider {
    public static readonly viewType = 'ricochet.chatView';

    private view?: vscode.WebviewView;
    private isLiveModeEnabled = false;

    private checkpointService?: ShadowCheckpointService;
    private sessionService: SessionService;
    private chatService?: ChatService;
    private mcpService?: McpService;
    private agentService: AgentService;
    private pendingPermissionRequests: Map<string, (response: string) => void> = new Map();

    constructor(
        private readonly context: vscode.ExtensionContext,
        private readonly core: CoreProcess
    ) {
        this.sessionService = new SessionService(context);
        this.agentService = new AgentService(context, this.core, (msg) => this.postMessage(msg));

        // Initialize services
        this.chatService = new ChatService(this.core, (msg: any) => this.postMessage(msg), this.sessionService);
        this.mcpService = new McpService(this.context, (msg: any) => this.postMessage(msg));

        // Listen for core messages and forward to webview
        this.core.onMessage('chat_update', (payload) => {
            // Forward to ChatService so it can save history
            this.chatService?.onChatUpdate(payload);
        });

        this.core.onMessage('live_mode_status', (payload) => {
            this.postMessage({ type: 'live_mode_status', payload });
        });

        // Forward Ether activity events (receiving, processing, responding)
        this.core.onMessage('ether_activity', (payload) => {
            this.postMessage({ type: 'ether_activity', payload });
        });

        this.core.onMessage('show_message', (payload: any) => {
            const { level, text } = payload;
            switch (level) {
                case 'error':
                    vscode.window.showErrorMessage(text);
                    break;
                case 'warning':
                    vscode.window.showWarningMessage(text);
                    break;
                case 'info':
                    vscode.window.showInformationMessage(text);
                    break;
            }
        });

        this.core.onMessage('mode_changed', (payload) => {
            this.postMessage({ type: 'mode_changed', payload });
        });

        // Handle synchronous requests from the core
        // Handle synchronous requests from the core
        this.core.onRequest('ask_user', (payload: any) => {
            const { question } = payload;

            // Create a promise that will be resolved when the webview responds
            return new Promise((resolve) => {
                const requestId = Date.now().toString(); // Simple ID for this interaction

                // Store the resolver to be called later
                this.pendingPermissionRequests.set(requestId, resolve);

                // Ask the webview to show the permission UI
                this.postMessage({
                    type: 'request_permission',
                    payload: {
                        id: requestId,
                        question
                    }
                });
            });
        });
    }

    resolveWebviewView(
        webviewView: vscode.WebviewView,
        _context: vscode.WebviewViewResolveContext,
        _token: vscode.CancellationToken
    ): void {
        this.view = webviewView;

        webviewView.webview.options = {
            enableScripts: true,
            localResourceRoots: [
                vscode.Uri.joinPath(this.context.extensionUri, 'webview-dist')
            ]
        };

        webviewView.webview.html = this.getWebviewContent(webviewView.webview);

        // Initialize checkpoints
        this.initCheckpoints().catch(console.error);

        // Handle messages from webview
        webviewView.webview.onDidReceiveMessage(async (message) => {
            console.log(`[Webview] Received message: ${message.type}`);
            // Forward everything to services
            await this.chatService?.handleMessage(message);
            await this.mcpService?.handleMessage(message);

            switch (message.type) {
                case 'permission_response':
                    const { id, answer } = message.payload;
                    const resolve = this.pendingPermissionRequests.get(id);
                    if (resolve) {
                        resolve(answer);
                        this.pendingPermissionRequests.delete(id);
                    }
                    break;

                case 'send_message':
                case 'toggle_live_mode':
                case 'clear_chat':
                case 'execute_command':
                    // Handled by ChatService
                    break;
                case 'get_mcp_servers':
                    try {
                        const mcpHub = await McpServerManager.getInstance(this.context);
                        const servers = mcpHub.getServers();
                        this.view?.webview.postMessage({ type: 'mcp_servers', payload: { servers } });
                    } catch (e) {
                        console.error('Failed to get MCP servers:', e);
                    }
                    break;

                case 'connect_mcp_server':
                    try {
                        const mcpHub = await McpServerManager.getInstance(this.context);
                        await mcpHub.connectToServer(message.payload.name, message.payload.config);
                        // Refresh list
                        const servers = mcpHub.getServers();
                        this.view?.webview.postMessage({ type: 'mcp_servers', payload: { servers } });
                    } catch (e) {
                        vscode.window.showErrorMessage(`Failed to connect MCP server: ${e}`);
                    }
                    break;

                case 'call_mcp_tool':
                    try {
                        const mcpHub = await McpServerManager.getInstance(this.context);
                        const result = await mcpHub.callTool(message.payload.serverName, message.payload.toolName, message.payload.args);
                        this.view?.webview.postMessage({
                            type: 'mcp_tool_result',
                            payload: {
                                id: message.payload.id,
                                result
                            }
                        });
                    } catch (e: any) {
                        this.view?.webview.postMessage({
                            type: 'mcp_tool_error',
                            payload: {
                                id: message.payload.id,
                                error: e.message
                            }
                        });
                    }
                    break;

                case 'checkpoint_init':
                    await this.initCheckpoints();
                    break;

                case 'save_checkpoint':
                    if (this.checkpointService) {
                        try {
                            const result = await this.checkpointService.saveCheckpoint(message.payload.message || 'Manual Checkpoint');
                            this.view?.webview.postMessage({ type: 'checkpoint_saved', payload: { commit: result?.commit } });
                        } catch (e: any) {
                            vscode.window.showErrorMessage(`Failed to save checkpoint: ${e.message}`);
                        }
                    }
                    break;

                case 'restore_checkpoint':
                    if (this.checkpointService) {
                        // Confirmation dialog
                        const ans = await vscode.window.showWarningMessage("Are you sure you want to restore? Current changes will be lost.", "Yes", "No");
                        if (ans === 'Yes') {
                            try {
                                await this.checkpointService.restoreCheckpoint(message.payload.hash);
                                vscode.window.showInformationMessage("Checkpoint restored.");
                                // Reload window or notify core?
                            } catch (e: any) {
                                vscode.window.showErrorMessage(`Failed to restore checkpoint: ${e.message}`);
                            }
                        }
                    }
                    break;

                case 'search_files':
                    // Handle file search request from webview
                    const query = message.payload.query || '';
                    if (!vscode.workspace.workspaceFolders) {
                        this.view?.webview.postMessage({ type: 'file_search_results', payload: [] });
                        return;
                    }

                    // Find files matching the query
                    // Use a glob pattern that matches the query in the filename
                    const globPattern = `**/*${query}*`;
                    const excludePattern = '**/{node_modules,.git,dist,out,build}/**';

                    vscode.workspace.findFiles(globPattern, excludePattern, 20).then(uris => {
                        const results = uris.map(uri => {
                            // Get workspace-relative path
                            const relativePath = vscode.workspace.asRelativePath(uri);
                            return {
                                path: relativePath,
                                name: uri.path.split('/').pop() || relativePath
                            };
                        });

                        this.view?.webview.postMessage({
                            type: 'file_search_results',
                            payload: results
                        });
                    });
                    break;

                case 'open_file':
                    const filePath = message.payload.path;
                    if (filePath) {
                        const fullPath = path.isAbsolute(filePath) ? filePath : path.join(vscode.workspace.workspaceFolders?.[0].uri.fsPath || '', filePath);
                        const uri = vscode.Uri.file(fullPath);
                        await vscode.window.showTextDocument(uri);
                    }
                    break;

                case 'audio_start':
                case 'audio_chunk':
                case 'audio_stop':
                    await this.core.send(message.type, message.payload || {});
                    break;
                // case 'get_state':
                //     // Handled by ChatService which respects session_id
                //     break;

                // Session Management
                case 'list_sessions':
                    const sessions = await this.sessionService.listSessions();
                    this.postMessage({ type: 'session_list', payload: { sessions } });
                    break;
                case 'create_session':
                    await this.createNewSession();
                    break;

                case 'load_session':
                    const sessionData = await this.sessionService.loadSession(message.payload.id);
                    if (sessionData) {
                        this.chatService?.setActiveSession(message.payload.id);

                        try {
                            // Hydrate backend with session history
                            await this.core.send('hydrate_session', {
                                session_id: message.payload.id,
                                messages: sessionData.messages
                            });
                        } catch (e) {
                            console.error('Failed to hydrate session:', e);
                        }

                        this.postMessage({ type: 'session_loaded', payload: { id: message.payload.id, ...sessionData } });
                    }
                    break;

                case 'delete_session':
                    await this.sessionService.deleteSession(message.payload.id);
                    const updatedSessions = await this.sessionService.listSessions();
                    this.postMessage({ type: 'session_list', payload: { sessions: updatedSessions } });
                    break;

                // Agent Manager Handlers
                case 'start_session':
                case 'cancel_session':
                case 'cancel_generation': // Alias for webview compatibility
                    await this.agentService.handleMessage(message);
                    break;

                case 'verify_telegram_token':
                    try {
                        const token = message.payload.token;
                        const response = await fetch(`https://api.telegram.org/bot${token}/getMe`);
                        const data = await response.json();
                        if (data.ok) {
                            this.postMessage({
                                type: 'bot_verification_result',
                                payload: {
                                    ok: true,
                                    username: data.result.username,
                                    firstName: data.result.first_name
                                }
                            });
                        } else {
                            this.postMessage({
                                type: 'bot_verification_result',
                                payload: { ok: false, error: data.description || 'Invalid token' }
                            });
                        }
                    } catch (e: any) {
                        this.postMessage({
                            type: 'bot_verification_result',
                            payload: { ok: false, error: 'Failed to verify: ' + e.message }
                        });
                    }
                    break;

                case 'get_settings':
                    try {
                        const settings = await this.core.send('get_settings', {});
                        this.postMessage({ type: 'settings_loaded', payload: settings });
                    } catch (e) {
                        console.error('Failed to get settings:', e);
                    }
                    break;

                case 'get_models':
                    try {
                        const payload = await this.core.send('get_models', {});
                        this.postMessage({ type: 'models', payload });
                    } catch (e) {
                        console.error('Failed to get models:', e);
                    }
                    break;

                case 'get_live_mode_status':
                    // Fire and forget request to core
                    this.core.send('get_live_mode_status', {}).catch(e => console.error('Error fetching live status:', e));
                    break;

                case 'save_settings':
                    try {
                        await this.core.send('save_settings', message.payload);
                        vscode.window.showInformationMessage('Settings saved');
                    } catch (e) {
                        vscode.window.showErrorMessage('Failed to save settings');
                    }
                    break;

                case 'auto_approve_settings':
                    try {
                        // Forward as partial save_settings
                        await this.core.send('save_settings', { auto_approval: message.payload });
                    } catch (e) {
                        console.error('Failed to sync auto-approve settings:', e);
                    }
                    break;

                case 'set_auto_approve':
                    try {
                        // 1. Get current settings
                        const currentSettings: any = await this.core.send('get_settings', {});
                        const autoApproval = currentSettings.auto_approval || {};

                        // 2. Patch settings
                        if (message.payload.commands) {
                            autoApproval.execute_safe_commands = true;
                            autoApproval.execute_all_commands = true;
                        }

                        // 3. Save
                        await this.core.send('save_settings', { auto_approval: autoApproval });

                        // 4. Resolve all pending requests since user said "Always"
                        if (message.payload.commands) {
                            for (const [id, resolve] of this.pendingPermissionRequests.entries()) {
                                resolve('yes'); // Assuming 'yes' is the approval string expected by Core
                            }
                            this.pendingPermissionRequests.clear();
                        }

                        // 5. Broadcast update so Panel refreshes
                        const newSettings = await this.core.send('get_settings', {});
                        this.postMessage({ type: 'settings_loaded', payload: newSettings });

                        vscode.window.showInformationMessage('Auto-approve enabled. Resuming commands...');
                    } catch (e) {
                        console.error('Failed to set auto-approve:', e);
                        vscode.window.showErrorMessage('Failed to enable auto-approve');
                    }
                    break;

                case 'test_telegram':
                    try {
                        const { token, chatId } = message.payload as { token: string; chatId: number };
                        const response = await fetch(
                            `https://api.telegram.org/bot${token}/sendMessage`,
                            {
                                method: 'POST',
                                headers: { 'Content-Type': 'application/json' },
                                body: JSON.stringify({
                                    chat_id: chatId,
                                    text: '✅ *Ricochet Ether Connected!*\n\nYour IDE is now paired with this chat.',
                                    parse_mode: 'Markdown'
                                })
                            }
                        );
                        const data = await response.json();
                        this.postMessage({
                            type: 'test_telegram_result',
                            payload: { ok: data.ok }
                        });
                    } catch (e) {
                        this.postMessage({
                            type: 'test_telegram_result',
                            payload: { ok: false }
                        });
                    }
                    break;

                default:
            }
        });
    }

    private async initCheckpoints() {
        if (this.checkpointService) {
            this.view?.webview.postMessage({
                type: 'checkpoint_initialized',
                payload: { baseHash: this.checkpointService.baseHash || '' }
            });
            return;
        }

        const workspaceFolders = vscode.workspace.workspaceFolders;
        if (!workspaceFolders || workspaceFolders.length === 0) {
            return;
        }
        const workspaceDir = workspaceFolders[0].uri.fsPath;
        const globalStorageDir = this.context.globalStorageUri.fsPath;
        const taskId = "default-session"; // TODO: mult-session support

        // Hash workspace to avoid collision if multiple Workspaces use same taskId (though taskId should be unique)
        // Roo uses tasks/{taskId}/checkpoints. We can use simplified path for now.
        const checkpointsDir = path.join(globalStorageDir, "checkpoints", nodeCrypto.createHash('md5').update(workspaceDir).digest('hex'));

        this.checkpointService = new ShadowCheckpointService(
            taskId,
            checkpointsDir,
            workspaceDir,
            (msg) => console.log(`[Checkpoint] ${msg}`)
        );

        await this.checkpointService.initShadowGit();

        // Send confirmation to webview
        this.view?.webview.postMessage({
            type: 'checkpoint_initialized',
            payload: { baseHash: this.checkpointService.baseHash || '' }
        });

        // Listen for internal events
        this.checkpointService.on('checkpoint', (data) => {
            this.view?.webview.postMessage({ type: 'checkpoint_update', payload: data });
        });
    }

    // Public methods for extension.ts
    async clearChat(): Promise<void> {
        await this.chatService?.handleMessage({ type: 'clear_chat' });
    }

    async createNewSession(): Promise<void> {
        if (vscode.workspace.workspaceFolders && vscode.workspace.workspaceFolders.length > 0) {
            const newId = await this.sessionService.createSession(vscode.workspace.workspaceFolders[0].uri.fsPath);
            this.chatService?.setActiveSession(newId);

            // Hydrate core with empty session to reset context
            try {
                await this.core.send('hydrate_session', {
                    session_id: newId,
                    messages: []
                });
            } catch (e) {
                console.error('Failed to hydrate new session:', e);
            }

            this.postMessage({ type: 'session_created', payload: { id: newId } });
        }
    }

    async toggleLiveMode(): Promise<void> {
        await this.chatService?.handleMessage({ type: 'toggle_live_mode' });
    }

    async openSettings(): Promise<void> {
        this.postMessage({ type: 'open_settings' });
    }

    async openAgent(): Promise<void> {
        this.postMessage({ type: 'open_agent' });
    }

    async openHistory(): Promise<void> {
        this.postMessage({ type: 'open_history' });
    }

    private postMessage(message: { type: string; payload?: unknown }): void {
        this.view?.webview.postMessage(message);
    }

    private getWebviewContent(webview: vscode.Webview): string {
        const scriptUri = webview.asWebviewUri(
            vscode.Uri.joinPath(this.context.extensionUri, 'webview-dist', 'main.js')
        );
        const styleUri = webview.asWebviewUri(
            vscode.Uri.joinPath(this.context.extensionUri, 'webview-dist', 'main.css')
        );

        const nonce = this.getNonce();

        return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src ${webview.cspSource} 'unsafe-inline'; script-src 'nonce-${nonce}'; font-src ${webview.cspSource};">
  <link href="${styleUri}" rel="stylesheet">
  <title>Ricochet</title>
</head>
<body>
  <div id="root"></div>
  <script nonce="${nonce}" src="${scriptUri}"></script>
</body>
</html>`;
    }

    private getNonce(): string {
        let text = '';
        const possible = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
        for (let i = 0; i < 32; i++) {
            text += possible.charAt(Math.floor(Math.random() * possible.length));
        }
        return text;
    }
}
