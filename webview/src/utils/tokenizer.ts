import { getEncoding } from 'js-tiktoken';

// Singleton encoder instance for performance
// cl100k_base is used by GPT-4 and GPT-3.5-turbo
const encoding = getEncoding('cl100k_base');

export function countTokens(text: string): number {
    try {
        return encoding.encode(text).length;
    } catch (e) {
        console.warn('Token counting error:', e);
        return 0;
    }
}
