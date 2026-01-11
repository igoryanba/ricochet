import { useState, useEffect } from 'react';
import { useAgentStateMachine } from '../../hooks/useAgentStateMachine';
import { SessionState } from '../../services/state-machine/sessionStateMachine';
import { useVSCodeApi } from '../../hooks/useVSCodeApi';
import { AgentLog } from './AgentLog';

export function AgentView() {
    const { state, uiState, context, send, reset } = useAgentStateMachine();
    const { postMessage, onMessage } = useVSCodeApi();
    const [prompt, setPrompt] = useState('');

    // Listen for backend events
    useEffect(() => {
        const unsubscribe = onMessage((message) => {
            switch (message.type) {
                case 'session_created':
                    send({ type: 'session_created', sessionId: (message.payload as any).sessionId });
                    break;
                case 'api_req_started':
                    send({ type: 'api_req_started' });
                    break;
                case 'say_text':
                    send({ type: 'say_text', payload: message.payload as any });
                    break;
                case 'ask_tool':
                    send({ type: 'ask_tool', payload: message.payload as any });
                    break;
                case 'ask_completion_result':
                    send({ type: 'ask_completion_result' });
                    break;
                case 'process_error': // If backend sends error
                    // send({ type: 'process_error', error: message.payload.error });
                    break;
            }
        });
        return () => { unsubscribe(); };
    }, [onMessage, send]);

    const handleStart = () => {
        if (!prompt.trim()) return;

        // 1. Update local state
        send({ type: 'start_session' });

        // 2. Notify backend to start actual work
        postMessage({
            type: 'start_session',
            payload: { prompt }
        });
    };

    const handleCancel = () => {
        send({ type: 'cancel_session' });
        postMessage({ type: 'cancel_session' });
    };

    return (
        <div className="chat-view h-screen flex flex-col">
            <div className="header p-4 border-b border-vscode-widget-border">
                <h2 className="font-semibold">Autonomous Agent</h2>
                <div className="text-xs text-vscode-descriptionForeground mt-1 flex items-center gap-2">
                    <div className={`w-2 h-2 rounded-full ${state === 'streaming' ? 'bg-green-500 animate-pulse' : 'bg-gray-500'}`} />
                    Status: {state}
                </div>
            </div>

            {state === SessionState.idle ? (
                <div className="p-6 flex flex-col gap-4 flex-1 justify-center max-w-md mx-auto w-full">
                    <div className="text-center mb-4">
                        <div className="w-12 h-12 bg-vscode-button-background rounded-full mx-auto flex items-center justify-center mb-3">
                            <span className="text-2xl">🤖</span>
                        </div>
                        <h3 className="text-lg font-medium">What can I do for you?</h3>
                        <p className="text-sm text-vscode-descriptionForeground opacity-80">
                            I can browse files, run commands, and edit code autonomously.
                        </p>
                    </div>

                    <textarea
                        className="vscode-input p-3 rounded"
                        style={{ minHeight: '120px', resize: 'none' }}
                        placeholder="e.g. Refactor the utils folder..."
                        value={prompt}
                        onChange={(e) => setPrompt(e.target.value)}
                    />
                    <button className="vscode-button py-2.5" onClick={handleStart} disabled={!prompt.trim()}>
                        Start Mission
                    </button>
                </div>
            ) : (
                <div className="flex-1 flex flex-col min-h-0">
                    {/* Log View */}
                    <AgentLog logs={context.logs} />

                    {/* Controls Footer */}
                    <div className="p-4 border-t border-vscode-widget-border bg-vscode-editor-background">
                        <div className="flex justify-between items-center">
                            {uiState.showSpinner && <span className="text-xs opacity-50 animate-pulse">Thinking...</span>}

                            <div className="flex gap-2">
                                {uiState.showCancelButton && (
                                    <button className="vscode-button secondary text-xs py-1" onClick={handleCancel}>
                                        Stop
                                    </button>
                                )}
                                {state === SessionState.completed && (
                                    <button className="vscode-button text-xs py-1" onClick={reset}>
                                        New Task
                                    </button>
                                )}
                                {(state === SessionState.stopped || state === SessionState.error) && (
                                    <button className="vscode-button text-xs py-1" onClick={reset}>
                                        Reset
                                    </button>
                                )}
                            </div>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
