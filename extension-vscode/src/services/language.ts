import * as vscode from 'vscode';
import { CoreProcess } from '../core-process';

interface Diagnostic {
    file: string;
    line: number;
    message: string;
    severity: string;
}

interface DefinitionLocation {
    file: string;
    start_line: number;
    end_line: number;
}

export class LanguageService {
    constructor(private coreProcess: CoreProcess) {
        this.registerHandlers();
    }

    private registerHandlers() {
        this.coreProcess.onRequest('get_diagnostics', async (payload: any) => {
            return this.getDiagnostics(payload.path);
        });

        this.coreProcess.onRequest('get_definitions', async (payload: any) => {
            return this.getDefinitions(payload.path, payload.line, payload.character);
        });
    }

    private async getDiagnostics(filePath: string): Promise<Diagnostic[]> {
        const uri = vscode.Uri.file(filePath);
        const diagnostics = vscode.languages.getDiagnostics(uri);

        return diagnostics.map(d => ({
            file: filePath,
            line: d.range.start.line + 1, // 1-indexed for agent
            message: d.message,
            severity: this.mapSeverity(d.severity)
        }));
    }

    private mapSeverity(severity: vscode.DiagnosticSeverity): string {
        switch (severity) {
            case vscode.DiagnosticSeverity.Error: return 'Error';
            case vscode.DiagnosticSeverity.Warning: return 'Warning';
            case vscode.DiagnosticSeverity.Information: return 'Information';
            case vscode.DiagnosticSeverity.Hint: return 'Hint';
            default: return 'Unknown';
        }
    }

    private async getDefinitions(filePath: string, line: number, character: number): Promise<DefinitionLocation[]> {
        const uri = vscode.Uri.file(filePath);
        // VS Code uses 0-indexed lines
        const position = new vscode.Position(line - 1, character);

        try {
            const result = await vscode.commands.executeCommand<vscode.Location[] | vscode.LocationLink[]>(
                'vscode.executeDefinitionProvider',
                uri,
                position
            );

            if (!result) {
                return [];
            }

            const locations: DefinitionLocation[] = [];

            if (Array.isArray(result)) {
                for (const item of result) {
                    if (item instanceof vscode.Location) {
                        locations.push({
                            file: item.uri.fsPath,
                            start_line: item.range.start.line + 1,
                            end_line: item.range.end.line + 1
                        });
                    } else {
                        // LocationLink
                        locations.push({
                            file: item.targetUri.fsPath,
                            start_line: item.targetRange.start.line + 1,
                            end_line: item.targetRange.end.line + 1
                        });
                    }
                }
            }
            return locations;

        } catch (error) {
            console.error('Failed to get definitions:', error);
            return [];
        }
    }
}
