import * as vscode from 'vscode';
import { WebviewProvider } from './webview-provider';
import { CoreProcess } from './core-process';

let coreProcess: CoreProcess | undefined;

export async function activate(context: vscode.ExtensionContext) {
    console.log('Ricochet extension activating...');

    // Get workspace root path
    const workspacePath = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath || context.extensionPath;
    console.log(`Starting CoreProcess with workspace: ${workspacePath}`);

    // Start the Go core process
    coreProcess = new CoreProcess(workspacePath, context.extensionPath);
    await coreProcess.start();

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
            await webviewProvider.clearChat();
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

    console.log('Ricochet extension activated');
}

export async function deactivate() {
    console.log('Ricochet extension deactivating...');

    if (coreProcess) {
        await coreProcess.stop();
    }

    console.log('Ricochet extension deactivated');
}
