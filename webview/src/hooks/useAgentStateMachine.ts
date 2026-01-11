import { useState, useRef, useCallback } from 'react';
import { createSessionStateMachine, SessionState, SessionEvent } from '../services/state-machine/sessionStateMachine';

export function useAgentStateMachine() {
    const machineRef = useRef(createSessionStateMachine());
    const [state, setState] = useState<SessionState>(machineRef.current.getState());
    const [uiState, setUiState] = useState(machineRef.current.getUiState());
    const [context, setContext] = useState(machineRef.current.getContext());

    const send = useCallback((event: SessionEvent) => {
        machineRef.current.send(event);
        // Sync state after transition
        setState(machineRef.current.getState());
        setUiState(machineRef.current.getUiState());
        setContext(machineRef.current.getContext());
    }, []);

    const reset = useCallback(() => {
        machineRef.current.reset();
        setState(machineRef.current.getState());
        setUiState(machineRef.current.getUiState());
        setContext(machineRef.current.getContext());
    }, []);

    return {
        state,
        uiState,
        context,
        send,
        reset
    };
}
