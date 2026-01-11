import * as vscode from 'vscode';
import { CoreProcess } from '../../core-process';
import { McpServerManager } from '../mcp/McpServerManager';

interface ToolCall {
    id: string;
    name: string;
    arguments: Record<string, unknown>;
    status: 'pending' | 'running' | 'completed' | 'error';
    result?: string;
}

interface ChatMessage {
    id: string;
    role: string;
    content: string;
    toolCalls?: ToolCall[];
    isStreaming?: boolean;
}

export class AgentService {
    private activeSessionId: string | null = null;
    private processedToolCallIds: Set<string> = new Set();

    constructor(
        private readonly context: vscode.ExtensionContext,
        private readonly core: CoreProcess,
        private readonly postMessage: (message: any) => void
    ) {
        // Listen to core stream
        this.core.onMessage('chat_update', (payload: any) => this.handleChatUpdate(payload));
    }

    public async handleMessage(message: any) {
        switch (message.type) {
            case 'start_session':
                await this.startSession(message.payload);
                break;
            case 'cancel_session':
                await this.cancelSession();
                break;
        }
    }

    private async startSession(payload?: { prompt: string }) {
        // Generate new Session ID
        this.activeSessionId = crypto.randomUUID();
        this.processedToolCallIds.clear();
        let prompt = payload?.prompt || "Start autonomous task.";

        // Notify frontend
        this.postMessage({
            type: 'session_created',
            sessionId: this.activeSessionId
        });

        // Notify start
        this.postMessage({ type: 'api_req_started' });

        // Phase 10: Inject Tools
        const mcpHub = await McpServerManager.getInstance(this.context);
        const servers = mcpHub.getServers();
        const tools = servers.flatMap(s => s.tools || []);

        const systemPrompt = `You are an autonomous AI agent. You have access to the following tools:\n` +
            tools.map(t => `- ${t.name}: ${t.description} (Args: ${JSON.stringify(t.inputSchema)})`).join('\n') +
            `\n\nWhen you need to use a tool, output a JSON block with "toolCalls": [{ "name": "...", "id": "uuid", "arguments": { ... } }]. ` +
            `Wait for the result in the next message using the same ID. verify your changes.`;

        // Send initial Prompt to Core
        await this.core.send('chat_message', {
            session_id: this.activeSessionId,
            content: `${systemPrompt}\n\nTask: ${prompt}`,
            role: 'user'
        });
    }

    private async handleChatUpdate(payload: { message: ChatMessage; session_id?: string }) {
        // Ignroe updates not for this agent session
        if (payload.session_id !== this.activeSessionId) return;

        const msg = payload.message;

        // Forward to UI to show progress (streaming text)
        // The AgentView should eventually display the conversation
        // For now, we rely on 'say_text' for status updates in the mocked UI, 
        // but let's also send the raw update so we can maybe render it later.
        // this.postMessage({ type: 'chat_update', payload }); 

        // If we have text content, show it as 'say_text'
        if (msg.content && msg.isStreaming) {
            this.postMessage({
                type: 'say_text',
                payload: { text: msg.content, partial: true }
            });
        }

        // Check for Tool Calls
        if (msg.toolCalls && msg.toolCalls.length > 0) {
            for (const tool of msg.toolCalls) {
                // Only execute if not processed and ready (assuming 'pending' means ready to execute)
                // In some systems, 'pending' means waiting for approval.
                if (tool.status === 'pending' && !this.processedToolCallIds.has(tool.id)) {
                    await this.executeTool(tool);
                }
            }
        }

        // Detect Completion (Stop)
        if (!msg.isStreaming && (!msg.toolCalls || msg.toolCalls.length === 0)) {
            this.postMessage({ type: 'ask_completion_result' }); // Marks as done in UI
        }
    }

    private async executeTool(tool: ToolCall) {
        this.processedToolCallIds.add(tool.id);

        try {
            // Notify UI we are waiting/executing
            this.postMessage({
                type: 'ask_tool',
                payload: { toolId: tool.id, name: tool.name, args: tool.arguments, partial: false }
            });

            // Auto-Approve & Execute (Phase 10 goal)
            const mcpHub = await McpServerManager.getInstance(this.context);

            // Assume tool name format "server_name:tool_name" or just "tool_name"
            // For now, simple lookup or pass to hub. 
            // If the tool name relies on a specific server not encoded in the name, we might need logic.
            // But McpServerManager usually needs serverName. 
            // Let's assume the LLM generates "server__tool" or we search.
            // actually mcpHub.callTool needs (serverName, toolName, args).

            // HACK: We need to find which server has this tool.
            // We'll iterate all servers.
            const servers = mcpHub.getServers();
            let foundServer = '';
            let realToolName = tool.name;

            for (const s of servers) {
                if (s.tools?.find(t => t.name === tool.name)) {
                    foundServer = s.name;
                    break;
                }
            }

            if (!foundServer) {
                throw new Error(`Tool ${tool.name} not found in any connected MCP server.`);
            }

            const result = await mcpHub.callTool(foundServer, realToolName, tool.arguments);

            // Send Result back to Core
            await this.core.send('chat_message', {
                session_id: this.activeSessionId,
                content: `Tool '${tool.name}' Output:\n${JSON.stringify(result, null, 2)}`,
                role: 'user' // Masquerade as user providing the result
            });

        } catch (error: any) {
            console.error('Agent Tool Execution Failed:', error);
            // Report error back to Core
            await this.core.send('chat_message', {
                session_id: this.activeSessionId,
                content: `Tool '${tool.name}' Failed: ${error.message}`,
                role: 'user'
            });
        }
    }

    private async cancelSession() {
        if (this.activeSessionId) {
            // Maybe send a 'stop' signal to Core if supported
            this.activeSessionId = null;
        }
    }
}
