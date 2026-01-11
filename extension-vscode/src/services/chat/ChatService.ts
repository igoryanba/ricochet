import * as vscode from 'vscode';
import { CoreProcess } from '../../core-process';
import { parseSourceCodeDefinitionsForFile } from '../tree-sitter';
import { DiffService } from '../diff/DiffService';
import { SessionService } from '../session/SessionService';

export class ChatService {
    private isLiveModeEnabled = false;
    private diffService: DiffService;
    private activeSessionId: string | null = null;

    constructor(
        private readonly core: CoreProcess,
        private readonly postMessage: (msg: any) => void,
        private readonly sessionService: SessionService
    ) {
        this.diffService = new DiffService();
    }

    public setActiveSession(sessionId: string) {
        this.activeSessionId = sessionId;
    }

    public async handleMessage(message: any): Promise<void> {
        switch (message.type) {
            case 'send_message':
                // Save user message to session
                if (this.activeSessionId) {
                    await this.sessionService.appendMessage(this.activeSessionId, {
                        role: 'user',
                        content: message.payload.content,
                        timestamp: Date.now()
                    });
                }

                await this.core.send('chat_message', message.payload);
                break;

            case 'toggle_live_mode':
                await this.toggleLiveMode();
                break;

            case 'clear_chat':
                await this.clearChat();
                break;

            case 'execute_command':
                this.executeCommand(message.payload.command);
                break;

            case 'search_files':
                await this.searchFiles(message.payload.query);
                break;

            case 'parse_file':
                await this.parseFile(message.payload.path);
                break;

            case 'audio_start':
            case 'audio_chunk':
            case 'audio_stop':
                await this.core.send(message.type, message.payload || {});
                break;

            case 'get_state':
                const state = await this.core.send('get_state', {});
                this.postMessage({ type: 'state', payload: state });
                break;

            case 'show_native_diff':
                try {
                    const { path, newContent, targetContent } = message.payload as any;
                    if (targetContent !== undefined) {
                        await this.diffService.showPartialDiff(path, targetContent, newContent);
                    } else {
                        await this.diffService.showDiff(path, newContent);
                    }
                } catch (e: any) {
                    vscode.window.showErrorMessage(`Failed to show diff: ${e.message}`);
                }
                break;
        }
    }

    public async onChatUpdate(payload: any): Promise<void> {
        // payload: { message: { role: 'assistant', content: '...', ... } }
        this.postMessage({ type: 'chat_update', payload });

        if (this.activeSessionId && payload.message && !payload.message.partial) {
            // If `isStreaming` is false, it might be the final block
            if (payload.message.isStreaming === false || payload.done === true) {
                await this.sessionService.appendMessage(this.activeSessionId, {
                    role: 'assistant',
                    content: payload.message.content,
                    timestamp: Date.now()
                });
            }
        }
    }

    private async toggleLiveMode(): Promise<void> {
        const requestedState = !this.isLiveModeEnabled;

        const result = await this.core.send('set_live_mode', {
            enabled: requestedState
        }) as { enabled?: boolean; error?: string };

        console.log('ChatService: Live Mode Result from Core:', result);

        this.isLiveModeEnabled = result?.enabled ?? false;

        this.postMessage({
            type: 'live_mode_status',
            payload: result
        });

        if (result?.error) {
            vscode.window.showWarningMessage(`Ether: ${result.error}`);
        } else {
            vscode.window.showInformationMessage(
                `Ether ${this.isLiveModeEnabled ? 'enabled' : 'disabled'}`
            );
        }
    }

    private async clearChat(): Promise<void> {
        await this.core.send('clear_chat', {});
        this.postMessage({ type: 'chat_cleared' });
    }

    private executeCommand(command: string): void {
        if (command) {
            const terminal = vscode.window.terminals.find(t => t.name === 'Ricochet')
                || vscode.window.createTerminal('Ricochet');
            terminal.show();
            terminal.sendText(command);
        }
    }

    private async searchFiles(query: string): Promise<void> {
        if (!vscode.workspace.workspaceFolders) {
            this.postMessage({ type: 'file_search_results', payload: [] });
            return;
        }

        const globPattern = `**/*${query || ''}*`;
        const excludePattern = '**/{node_modules,.git,dist,out,build,.next}/**';

        try {
            const uris = await vscode.workspace.findFiles(globPattern, excludePattern, 20);
            const results = uris.map(uri => {
                const relativePath = vscode.workspace.asRelativePath(uri);
                return {
                    path: relativePath,
                    name: uri.path.split('/').pop() || relativePath
                };
            });

            this.postMessage({
                type: 'file_search_results',
                payload: results
            });
        } catch (e) {
            console.error('File search failed:', e);
            this.postMessage({ type: 'file_search_results', payload: [] });
        }
    }

    private async parseFile(filePath: string): Promise<void> {
        try {
            let fullPath = filePath;
            if (!filePath.startsWith('/') && vscode.workspace.workspaceFolders) {
                fullPath = vscode.Uri.joinPath(vscode.workspace.workspaceFolders[0].uri, filePath).fsPath;
            }

            const definitions = await parseSourceCodeDefinitionsForFile(fullPath);
            this.postMessage({
                type: 'parse_file_result',
                payload: {
                    path: filePath,
                    definitions: definitions || 'No definitions found.'
                }
            });
        } catch (e: any) {
            this.postMessage({
                type: 'parse_file_error',
                payload: {
                    path: filePath,
                    error: e.message
                }
            });
        }
    }
}
