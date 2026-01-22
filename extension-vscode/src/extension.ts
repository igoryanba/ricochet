import * as vscode from 'vscode';
import { WebviewProvider } from './webview-provider';
import { CoreProcess } from './core-process';
import { LanguageService } from './services/language';

let coreProcess: CoreProcess | undefined;

export async function activate(context: vscode.ExtensionContext) {
    console.log('Ricochet extension activating...');

    // Get workspace root path
    const workspacePath = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath || context.extensionPath;
    console.log(`Starting CoreProcess with workspace: ${workspacePath}`);

    // Start the Go core process
    coreProcess = new CoreProcess(workspacePath, context.extensionPath);
    await coreProcess.start();

    // Initialize Language Service (LSP Bridge)
    new LanguageService(coreProcess);

    // Register webview provider
    const webviewProvider = new WebviewProvider(context, coreProcess);

    context.subscriptions.push(
        vscode.window.registerWebviewViewProvider(
            'ricochet.chatView',
            webviewProvider,
            { webviewOptions: { retainContextWhenHidden: true } }
        )
    );

    // Register commands
    context.subscriptions.push(
        vscode.commands.registerCommand('ricochet.newChat', async () => {
            await webviewProvider.createNewSession();
        })
    );

    context.subscriptions.push(
        vscode.commands.registerCommand('ricochet.toggleLiveMode', async () => {
            await webviewProvider.toggleLiveMode();
        })
    );

    context.subscriptions.push(
        vscode.commands.registerCommand('ricochet.openSettings', async () => {
            await webviewProvider.openSettings();
        })
    );

    context.subscriptions.push(
        vscode.commands.registerCommand('ricochet.openAgent', async () => {
            await webviewProvider.openAgent();
        })
    );

    context.subscriptions.push(
        vscode.commands.registerCommand('ricochet.openHistory', async () => {
            await webviewProvider.openHistory();
        })
    );

    // Register generic install command
    context.subscriptions.push(
        vscode.commands.registerCommand('ricochet.installCli', async () => {
            await installCli(context);
        })
    );

    // Check if CLI is installed on startup
    checkCliInstallation(context);

    console.log('Ricochet extension activated');
}

import { installCli } from './commands/installCli';
import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';

function checkCliInstallation(context: vscode.ExtensionContext) {
    const homeDir = os.homedir();
    const targetPath = path.join(homeDir, '.local', 'bin', 'ricochet');

    // Simple check: does the file exist?
    // In future we might check versions
    if (!fs.existsSync(targetPath)) {
        vscode.window.showInformationMessage(
            "Ricochet CLI is not installed in ~/.local/bin. Install it for terminal integration?",
            "Install",
            "Ignore"
        ).then(selection => {
            if (selection === "Install") {
                vscode.commands.executeCommand('ricochet.installCli');
            }
        });
    }
}

export async function deactivate() {
    console.log('Ricochet extension deactivating...');

    if (coreProcess) {
        await coreProcess.stop();
    }

    console.log('Ricochet extension deactivated');
}
