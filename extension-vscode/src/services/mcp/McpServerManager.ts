import * as vscode from "vscode"
import { McpHub } from "./McpHub"

export class McpServerManager {
    private static instance: McpHub | null = null

    static async getInstance(context: vscode.ExtensionContext): Promise<McpHub> {
        if (!this.instance) {
            this.instance = new McpHub(context)
        }
        return this.instance
    }
}
