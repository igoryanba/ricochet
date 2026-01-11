import * as vscode from 'vscode';
import { McpServerManager } from './McpServerManager';

export class McpService {
    constructor(
        private readonly context: vscode.ExtensionContext,
        private readonly postMessage: (msg: any) => void
    ) { }

    public async handleMessage(message: any): Promise<void> {
        switch (message.type) {
            case 'get_mcp_servers':
                await this.getServers();
                break;
            case 'connect_mcp_server':
                await this.connectServer(message.payload);
                break;
            case 'call_mcp_tool':
                await this.callTool(message.payload);
                break;
        }
    }

    private async getServers(): Promise<void> {
        try {
            const mcpHub = await McpServerManager.getInstance(this.context);
            const servers = mcpHub.getServers();
            this.postMessage({ type: 'mcp_servers', payload: { servers } });
        } catch (e) {
            console.error('Failed to get MCP servers:', e);
        }
    }

    private async connectServer(payload: { name: string, config: string }): Promise<void> {
        try {
            const mcpHub = await McpServerManager.getInstance(this.context);
            await mcpHub.connectToServer(payload.name, payload.config);
            // Refresh list
            const servers = mcpHub.getServers();
            this.postMessage({ type: 'mcp_servers', payload: { servers } });
        } catch (e) {
            vscode.window.showErrorMessage(`Failed to connect MCP server: ${e}`);
        }
    }

    private async callTool(payload: { id: string, serverName: string, toolName: string, args: any }): Promise<void> {
        try {
            const mcpHub = await McpServerManager.getInstance(this.context);
            const result = await mcpHub.callTool(payload.serverName, payload.toolName, payload.args);
            this.postMessage({
                type: 'mcp_tool_result',
                payload: {
                    id: payload.id,
                    result
                }
            });
        } catch (e: any) {
            this.postMessage({
                type: 'mcp_tool_error',
                payload: {
                    id: payload.id,
                    error: e.message
                }
            });
        }
    }
}
