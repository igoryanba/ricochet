import { useState } from 'react';
import { useMcp } from '../../hooks/useMcp';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '../ui/dialog';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select';
import { Server, Plug, Plus, RefreshCw, AlertCircle } from 'lucide-react';
import { cn } from '../../lib/utils';

export function McpView() {
    const { servers, isLoading, connectServer, refreshServers } = useMcp();
    const [isAddOpen, setIsAddOpen] = useState(false);

    // Form State
    const [name, setName] = useState('');
    const [type, setType] = useState<'stdio' | 'sse'>('stdio');
    const [command, setCommand] = useState('');
    const [args, setArgs] = useState('');
    const [url, setUrl] = useState('');

    const handleConnect = () => {
        const config = type === 'stdio'
            ? { command, args: args.split(' ').filter(Boolean) }
            : { url };

        connectServer(name, config);
        setIsAddOpen(false);
        // Reset form
        setName('');
        setCommand('');
        setArgs('');
        setUrl('');
    };

    return (
        <div className="flex flex-col h-full bg-vscode-editor-background text-vscode-fg p-4 space-y-4">
            <div className="flex items-center justify-between border-b border-vscode-border pb-4">
                <div className="flex items-center gap-2">
                    <Server className="text-ricochet-primary h-5 w-5" />
                    <h2 className="text-lg font-semibold cursor-default select-none">MCP Servers</h2>
                </div>
                <div className="flex items-center gap-2">
                    <Button variant="ghost" size="icon" onClick={refreshServers} title="Refresh Servers">
                        <RefreshCw className={cn("h-4 w-4", isLoading && "animate-spin")} />
                    </Button>
                    <Dialog open={isAddOpen} onOpenChange={setIsAddOpen}>
                        <DialogTrigger asChild>
                            <Button size="sm" className="gap-1">
                                <Plus className="h-3.5 w-3.5" />
                                Add Server
                            </Button>
                        </DialogTrigger>
                        <DialogContent>
                            <DialogHeader>
                                <DialogTitle>Connect New MCP Server</DialogTitle>
                            </DialogHeader>
                            <div className="flex flex-col gap-4 py-4">
                                <div className="space-y-2">
                                    <label className="text-xs font-medium text-muted-foreground">Server Name</label>
                                    <Input value={name} onChange={e => setName(e.target.value)} placeholder="e.g. My Worker" />
                                </div>
                                <div className="space-y-2">
                                    <label className="text-xs font-medium text-muted-foreground">Type</label>
                                    <Select value={type} onValueChange={(v: any) => setType(v as 'stdio' | 'sse')}>
                                        <SelectTrigger>
                                            <SelectValue />
                                        </SelectTrigger>
                                        <SelectContent>
                                            <SelectItem value="stdio">Stdio (Command)</SelectItem>
                                            <SelectItem value="sse">SSE (URL)</SelectItem>
                                        </SelectContent>
                                    </Select>
                                </div>
                                {type === 'stdio' ? (
                                    <>
                                        <div className="space-y-2">
                                            <label className="text-xs font-medium text-muted-foreground">Command</label>
                                            <Input value={command} onChange={e => setCommand(e.target.value)} placeholder="e.g. npx" />
                                        </div>
                                        <div className="space-y-2">
                                            <label className="text-xs font-medium text-muted-foreground">Args</label>
                                            <Input value={args} onChange={e => setArgs(e.target.value)} placeholder="e.g. -y @server/package" />
                                        </div>
                                    </>
                                ) : (
                                    <div className="space-y-2">
                                        <label className="text-xs font-medium text-muted-foreground">Server URL</label>
                                        <Input value={url} onChange={e => setUrl(e.target.value)} placeholder="http://localhost:3000/sse" />
                                    </div>
                                )}
                                <Button onClick={handleConnect} disabled={!name}>Connect</Button>
                            </div>
                        </DialogContent>
                    </Dialog>
                </div>
            </div>

            <div className="flex-1 overflow-y-auto space-y-2">
                {servers.length === 0 && (
                    <div className="flex flex-col items-center justify-center h-40 text-vscode-desc space-y-2">
                        <Plug className="h-8 w-8 opacity-20" />
                        <span className="text-sm">No servers connected</span>
                    </div>
                )}
                {servers.map(server => (
                    <div key={server.name} className="flex items-center justify-between p-3 rounded-md border border-vscode-border bg-vscode-editor-background hover:bg-vscode-list-hoverBackground transition-colors">
                        <div className="flex items-center gap-3">
                            <div className={cn(
                                "h-2 w-2 rounded-full",
                                server.status === 'connected' ? "bg-green-500 shadow-[0_0_8px_rgba(34,197,94,0.4)]" :
                                    server.status === 'connecting' ? "bg-yellow-500 animate-pulse" : "bg-red-500"
                            )} />
                            <div className="flex flex-col">
                                <span className="font-medium text-sm">{server.name}</span>
                                <span className="text-xs text-vscode-desc">
                                    {server.tools?.length || 0} tools • {server.resources?.length || 0} resources
                                </span>
                            </div>
                        </div>
                        {server.error && (
                            <div className="text-red-400" title={server.error}>
                                <AlertCircle className="h-4 w-4" />
                            </div>
                        )}
                        {/* Add Actions (Disconnect/Log) later */}
                    </div>
                ))}
            </div>
        </div>
    );
}
