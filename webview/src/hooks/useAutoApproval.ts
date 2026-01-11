import { useState, useCallback } from 'react';
import { useVSCodeApi } from './useVSCodeApi';

export interface AutoApprovalConfig {
    // Read operations
    readFiles: boolean;
    listFiles: boolean;
    searchFiles: boolean;

    // Write operations
    writeFiles: boolean;
    createFiles: boolean;
    deleteFiles: boolean;

    // Execute operations
    executeCommands: boolean;

    // Browser
    browserActions: boolean;

    // MCP
    mcpTools: boolean;

    // Master switch
    enabled: boolean;
}

const DEFAULT_CONFIG: AutoApprovalConfig = {
    readFiles: true,
    listFiles: true,
    searchFiles: true,
    writeFiles: false,
    createFiles: false,
    deleteFiles: false,
    executeCommands: false,
    browserActions: false,
    mcpTools: false,
    enabled: false
};

// Tool categories for auto-approval
const SAFE_TOOLS = new Set([
    'read_file',
    'list_files',
    'search_files',
    'view_file',
    'view_directory',
    'codebase_search'
]);

const WRITE_TOOLS = new Set([
    'write_to_file',
    'apply_diff',
    'replace_in_file',
    'insert_code_block'
]);

const CREATE_TOOLS = new Set([
    'create_file',
    'create_directory'
]);

const DELETE_TOOLS = new Set([
    'delete_file',
    'delete_directory'
]);

const EXECUTE_TOOLS = new Set([
    'execute_command',
    'run_terminal_command'
]);

const BROWSER_TOOLS = new Set([
    'browser_action',
    'take_screenshot'
]);

export function useAutoApproval() {
    const [config, setConfig] = useState<AutoApprovalConfig>(DEFAULT_CONFIG);
    const { postMessage } = useVSCodeApi();

    const updateConfig = useCallback((updates: Partial<AutoApprovalConfig>) => {
        setConfig(prev => {
            const next = { ...prev, ...updates };
            // Persist to extension
            postMessage({
                type: 'update_auto_approval',
                payload: next
            });
            return next;
        });
    }, [postMessage]);

    const toggleEnabled = useCallback(() => {
        updateConfig({ enabled: !config.enabled });
    }, [config.enabled, updateConfig]);

    /**
     * Check if a tool should be auto-approved based on current config
     */
    const shouldAutoApprove = useCallback((toolName: string): boolean => {
        if (!config.enabled) return false;

        // Safe read operations
        if (SAFE_TOOLS.has(toolName)) {
            return config.readFiles || config.listFiles || config.searchFiles;
        }

        // Write operations
        if (WRITE_TOOLS.has(toolName)) {
            return config.writeFiles;
        }

        // Create operations
        if (CREATE_TOOLS.has(toolName)) {
            return config.createFiles;
        }

        // Delete operations
        if (DELETE_TOOLS.has(toolName)) {
            return config.deleteFiles;
        }

        // Execute operations
        if (EXECUTE_TOOLS.has(toolName)) {
            return config.executeCommands;
        }

        // Browser operations
        if (BROWSER_TOOLS.has(toolName)) {
            return config.browserActions;
        }

        // MCP tools
        if (toolName.startsWith('mcp_')) {
            return config.mcpTools;
        }

        return false;
    }, [config]);

    /**
     * Get risk level for a tool
     */
    const getToolRiskLevel = useCallback((toolName: string): 'safe' | 'medium' | 'high' => {
        if (SAFE_TOOLS.has(toolName)) return 'safe';
        if (WRITE_TOOLS.has(toolName) || CREATE_TOOLS.has(toolName)) return 'medium';
        if (DELETE_TOOLS.has(toolName) || EXECUTE_TOOLS.has(toolName)) return 'high';
        return 'medium';
    }, []);

    return {
        config,
        updateConfig,
        toggleEnabled,
        shouldAutoApprove,
        getToolRiskLevel
    };
}

/**
 * Get human-readable category for a tool
 */
export function getToolCategory(toolName: string): string {
    if (SAFE_TOOLS.has(toolName)) return 'Read';
    if (WRITE_TOOLS.has(toolName)) return 'Write';
    if (CREATE_TOOLS.has(toolName)) return 'Create';
    if (DELETE_TOOLS.has(toolName)) return 'Delete';
    if (EXECUTE_TOOLS.has(toolName)) return 'Execute';
    if (BROWSER_TOOLS.has(toolName)) return 'Browser';
    if (toolName.startsWith('mcp_')) return 'MCP';
    return 'Other';
}
