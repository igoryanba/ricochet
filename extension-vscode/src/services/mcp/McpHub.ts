import { Client } from "@modelcontextprotocol/sdk/client/index.js"
import { StdioClientTransport, getDefaultEnvironment } from "@modelcontextprotocol/sdk/client/stdio.js"
import { SSEClientTransport } from "@modelcontextprotocol/sdk/client/sse.js"
import {
    CallToolResultSchema,
    ListResourcesResultSchema,
    ListResourceTemplatesResultSchema,
    ListToolsResultSchema,
    ReadResourceResultSchema,
} from "@modelcontextprotocol/sdk/types.js"
import * as vscode from "vscode"
import { z } from "zod"
import * as fs from "fs"
import * as path from "path"
import { McpServer, McpTool, McpResource, McpResourceTemplate, McpErrorEntry } from "../../types/mcp"
import { sanitizeMcpName } from "../../utils/mcp-name"

// Simple ReconnectingEventSource shim can be added here or imported if installed
import ReconnectingEventSource from "reconnecting-eventsource"

export type ConnectedMcpConnection = {
    type: "connected"
    server: McpServer
    client: Client
    transport: StdioClientTransport | SSEClientTransport
}

export type DisconnectedMcpConnection = {
    type: "disconnected"
    server: McpServer
    client: null
    transport: null
}

export type McpConnection = ConnectedMcpConnection | DisconnectedMcpConnection

const BaseConfigSchema = z.object({
    disabled: z.boolean().optional(),
    alwaysAllow: z.array(z.string()).default([]),
})

export const ServerConfigSchema = z.union([
    BaseConfigSchema.extend({
        command: z.string(),
        args: z.array(z.string()).optional(),
        env: z.record(z.string(), z.string()).optional(),
    }).strict(), // Stdio
    BaseConfigSchema.extend({
        url: z.string().url(),
    }).strict() // SSE/HTTP
])

export class McpHub {
    connections: McpConnection[] = []
    isConnecting: boolean = false
    private disposables: vscode.Disposable[] = []

    constructor(private context: vscode.ExtensionContext) {
        // Initialize from global state or settings if needed
    }

    public async connectToServer(name: string, config: any): Promise<void> {
        const sanitizedName = sanitizeMcpName(name)

        let client: Client
        let transport: StdioClientTransport | SSEClientTransport

        try {
            client = new Client(
                { name: "Ricochet", version: "1.0.0" },
                { capabilities: {} }
            )

            if (config.command) {
                transport = new StdioClientTransport({
                    command: config.command,
                    args: config.args,
                    env: {
                        ...getDefaultEnvironment(),
                        ...config.env
                    }
                })
            } else if (config.url) {
                transport = new SSEClientTransport(new URL(config.url), {
                    eventSourceInit: { withCredentials: false }
                })
            } else {
                throw new Error("Invalid config: missing command or url")
            }

            // Start transport
            await client.connect(transport)

            const connection: ConnectedMcpConnection = {
                type: "connected",
                server: {
                    name,
                    config: JSON.stringify(config),
                    status: "connected",
                    source: "global"
                },
                client,
                transport
            }

            // Fetch capabilities
            const tools = await client.request({ method: "tools/list" }, ListToolsResultSchema)
            connection.server.tools = (tools.tools || []).map(t => ({
                name: t.name,
                description: t.description,
                inputSchema: t.inputSchema as object,
            }))

            const resources = await client.request({ method: "resources/list" }, ListResourcesResultSchema)
            connection.server.resources = (resources.resources || []).map(r => ({
                uri: r.uri,
                name: r.name,
                mimeType: r.mimeType
            }))

            this.connections.push(connection)
            console.log(`[TCP] Connected to ${name}`)

        } catch (error: any) {
            console.error(`Failed to connect to ${name}:`, error)
            // Store disconnected state
            this.connections.push({
                type: "disconnected",
                server: {
                    name,
                    config: JSON.stringify(config),
                    status: "disconnected",
                    error: error.message
                },
                client: null,
                transport: null
            })
        }
    }

    public async callTool(serverName: string, toolName: string, args: any): Promise<any> {
        const connection = this.connections.find(c => c.server.name === serverName && c.type === "connected") as ConnectedMcpConnection
        if (!connection) {
            throw new Error(`Server ${serverName} not connected`)
        }

        const result = await connection.client.request(
            {
                method: "tools/call",
                params: {
                    name: toolName,
                    arguments: args
                }
            },
            CallToolResultSchema
        )
        return result
    }

    public async readResource(serverName: string, uri: string): Promise<any> {
        const connection = this.connections.find(c => c.server.name === serverName && c.type === "connected") as ConnectedMcpConnection
        if (!connection) {
            throw new Error(`Server ${serverName} not connected`)
        }

        const result = await connection.client.request(
            {
                method: "resources/read",
                params: { uri }
            },
            ReadResourceResultSchema
        )
        return result
    }

    public getServers(): McpServer[] {
        return this.connections.map(c => c.server)
    }

    dispose() {
        this.connections.forEach(c => {
            if (c.type === "connected") {
                c.client.close()
                c.transport.close()
            }
        })
        this.disposables.forEach(d => d.dispose())
    }
}
