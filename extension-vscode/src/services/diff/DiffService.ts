import * as vscode from 'vscode';
import * as path from 'path';
import * as fs from 'fs';
import * as os from 'os';
import * as crypto from 'crypto';

export class DiffService {
    private static tempDir = path.join(os.tmpdir(), 'ricochet-diff');

    constructor() {
        if (!fs.existsSync(DiffService.tempDir)) {
            fs.mkdirSync(DiffService.tempDir, { recursive: true });
        }
    }

    public async showDiff(filePath: string, newContent: string): Promise<void> {
        const workspaceFolders = vscode.workspace.workspaceFolders;
        if (!workspaceFolders) {
            throw new Error('No workspace folder open');
        }

        const fullPath = path.isAbsolute(filePath) ? filePath : path.join(workspaceFolders[0].uri.fsPath, filePath);

        if (!fs.existsSync(fullPath)) {
            // New file case or file not found
            const newFileUri = vscode.Uri.file(fullPath);
            await vscode.window.showTextDocument(newFileUri);
            return;
        }

        const originalContent = fs.readFileSync(fullPath, 'utf8');

        // Create temporary file for new content
        const fileName = path.basename(filePath);
        const folderName = crypto.createHash('md5').update(filePath).digest('hex').substring(0, 8);
        const tempFolderPath = path.join(DiffService.tempDir, folderName);

        if (!fs.existsSync(tempFolderPath)) {
            fs.mkdirSync(tempFolderPath, { recursive: true });
        }

        const tempFilePath = path.join(tempFolderPath, fileName);
        fs.writeFileSync(tempFilePath, newContent);

        const originalUri = vscode.Uri.file(fullPath);
        const proposedUri = vscode.Uri.file(tempFilePath);

        await vscode.commands.executeCommand(
            'vscode.diff',
            originalUri,
            proposedUri,
            `${fileName} (Proposed Changes)`
        );
    }

    public async showPartialDiff(filePath: string, targetContent: string, replacementContent: string): Promise<void> {
        const workspaceFolders = vscode.workspace.workspaceFolders;
        if (!workspaceFolders) {
            throw new Error('No workspace folder open');
        }

        const fullPath = path.isAbsolute(filePath) ? filePath : path.join(workspaceFolders[0].uri.fsPath, filePath);

        if (!fs.existsSync(fullPath)) {
            throw new Error('File not found for partial diff');
        }

        const originalContent = fs.readFileSync(fullPath, 'utf8');

        // Find and replace (simple string replacement for now)
        if (!originalContent.includes(targetContent)) {
            throw new Error('Target content not found in file');
        }

        const newContent = originalContent.replace(targetContent, replacementContent);

        // Use existing showDiff logic
        await this.showDiff(filePath, newContent);
    }

    public static cleanup(): void {
        if (fs.existsSync(DiffService.tempDir)) {
            try {
                fs.rmSync(DiffService.tempDir, { recursive: true, force: true });
            } catch (e) {
                console.error('Failed to cleanup diff temp dir:', e);
            }
        }
    }
}
