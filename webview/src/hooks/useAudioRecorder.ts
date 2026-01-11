import { useState, useRef, useCallback } from 'react';
import { useVSCodeApi } from './useVSCodeApi';

/**
 * Hook to record audio from the microphone and send chunks to the Extension.
 * Uses MediaRecorder with Opus codec.
 */
export function useAudioRecorder() {
    const { postMessage } = useVSCodeApi();
    const [isRecording, setIsRecording] = useState(false);
    const mediaRecorderRef = useRef<MediaRecorder | null>(null);

    const startRecording = useCallback(async () => {
        try {
            const stream = await navigator.mediaDevices.getUserMedia({ audio: true });

            // Note: Different browsers/OS support different mimeTypes. 
            // audio/webm is widely supported in VSCode's Chromium context.
            const mediaRecorder = new MediaRecorder(stream, {
                mimeType: 'audio/webm;codecs=opus'
            });
            mediaRecorderRef.current = mediaRecorder;

            mediaRecorder.ondataavailable = async (e) => {
                if (e.data.size > 0) {
                    const reader = new FileReader();
                    reader.onloadend = () => {
                        const base64data = (reader.result as string).split(',')[1];
                        postMessage({
                            type: 'audio_chunk',
                            payload: {
                                data: base64data,
                                format: 'webm'
                            }
                        });
                    };
                    reader.readAsDataURL(e.data);
                }
            };

            // Capture chunks every 500ms for smoother live transcription feel
            mediaRecorder.start(500);
            setIsRecording(true);

            postMessage({ type: 'audio_start' });
        } catch (err) {
            console.error('Failed to start recording:', err);
            setIsRecording(false);
        }
    }, [postMessage]);

    const stopRecording = useCallback(() => {
        if (mediaRecorderRef.current && mediaRecorderRef.current.state !== 'inactive') {
            mediaRecorderRef.current.stop();
            mediaRecorderRef.current.stream.getTracks().forEach(track => track.stop());
            setIsRecording(false);

            postMessage({ type: 'audio_stop' });
        }
    }, [postMessage]);

    const toggleRecording = useCallback(() => {
        if (isRecording) {
            stopRecording();
        } else {
            startRecording();
        }
    }, [isRecording, startRecording, stopRecording]);

    return { isRecording, startRecording, stopRecording, toggleRecording };
}
