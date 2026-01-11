export interface SessionMetadata {
    id: string;
    title: string;
    lastModified: number;
    messageCount: number;
    workspaceDir: string;
}

export interface SessionData {
    messages: any[]; // ChatMessage[]
    todos: any[];    // Todo[]
}
