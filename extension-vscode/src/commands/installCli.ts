import * as vscode from 'vscode';
import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';

export async function installCli(context: vscode.ExtensionContext) {
    // 1. Locate the binary (bundled with extension)
    // In dev: likely in core/ricochet
    // In prod: likely in bin/ricochet
    // We need a robust way to find it. For now, let's assume standard packaging.
    let extensionBinPath = path.join(context.extensionPath, 'bin', 'ricochet');

    // Dev fallback (if running extension from source and binary is not copied yet)
    if (!fs.existsSync(extensionBinPath)) {
        // Try looking in the core directory relative to extension root if in monorepo
        const devBinPath = path.join(context.extensionPath, '..', 'core', 'ricochet');
        if (fs.existsSync(devBinPath)) {
            extensionBinPath = devBinPath;
        } else {
            vscode.window.showErrorMessage(`Ricochet binary not found at ${extensionBinPath}`);
            return;
        }
    }

    // 2. Determine target path
    const homeDir = os.homedir();
    const localBin = path.join(homeDir, '.local', 'bin');
    const targetPath = path.join(localBin, 'ricochet');

    // Ensure ~/.local/bin exists
    if (!fs.existsSync(localBin)) {
        try {
            fs.mkdirSync(localBin, { recursive: true });
        } catch (error) {
            vscode.window.showErrorMessage(`Failed to create ${localBin}: ${error}`);
            return;
        }
    }

    // 3. Create Symlink
    try {
        // Remove existing link if present
        if (fs.existsSync(targetPath)) {
            fs.unlinkSync(targetPath);
        }

        fs.symlinkSync(extensionBinPath, targetPath);
        fs.chmodSync(extensionBinPath, '755'); // Ensure executable

        vscode.window.showInformationMessage(`Successfully installed 'ricochet' to ${targetPath}!`);

        // Optional: Check if ~/.local/bin is in PATH
        if (!process.env.PATH?.includes('.local/bin')) {
            vscode.window.showWarningMessage(`Note: ${localBin} is not in your PATH. You may need to add it.`);
        }

    } catch (error) {
        console.error(error);

        // Fallback: Copy to clipboard?
        const action = await vscode.window.showErrorMessage(
            `Failed to install CLI: ${error}. Try manual installation?`,
            'Copy Command'
        );

        if (action === 'Copy Command') {
            const cmd = `ln -s "${extensionBinPath}" /usr/local/bin/ricochet`;
            await vscode.env.clipboard.writeText(cmd);
            vscode.window.showInformationMessage('Command copied! Run it in your terminal (may require sudo).');
        }
    }
}
