import { useState, useEffect } from 'react';
import { Key, Radio, Info, ChevronLeft, Zap, CheckCircle, XCircle, Loader2, Copy, Github, Linkedin, Twitter, Heart, Coffee, CreditCard } from 'lucide-react';
import { RicochetLogo } from '../icons/RicochetLogo';
import { useVSCodeApi } from '../../hooks/useVSCodeApi';
import { Input } from '../ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select';

interface SettingsProps {
    onClose: () => void;
}

// Tab definitions
const TABS = [
    { id: 'api', label: 'API', icon: Key },
    { id: 'automations', label: 'Automations', icon: Zap },
    { id: 'live', label: 'Ether', icon: Radio },
    { id: 'about', label: 'About', icon: Info },
] as const;

interface AutoApprovalSettings {
    enabled: boolean;
    read_files: boolean;
    read_files_external: boolean;
    edit_files: boolean;
    edit_files_external: boolean;
    execute_safe_commands: boolean;
    execute_all_commands: boolean;
    use_browser: boolean;
    use_mcp: boolean;
}

interface ContextSettings {
    auto_condense: boolean;
    condense_threshold: number;
    sliding_window_size: number;
    show_context_indicator: boolean;
    enable_checkpoints: boolean;
    checkpoint_on_writes: boolean;
}

type TabId = typeof TABS[number]['id'];

interface BotInfo {
    ok: boolean;
    username?: string;
    firstName?: string;
    error?: string;
}

/**
 * Settings panel — Kilocode-style with vertical tabs.
 */
export function Settings({ onClose }: SettingsProps) {
    const [activeTab, setActiveTab] = useState<TabId>('api');

    // API State
    const [apiKeys, setApiKeys] = useState<Record<string, string>>({});
    const [provider, setProvider] = useState<string>('anthropic');
    const [model, setModel] = useState('');

    // Dynamic providers from backend
    interface ModelInfo {
        id: string;
        name: string;
        contextWindow: number;
        inputPrice: number;
        outputPrice: number;
        isFree: boolean;
        supportsTools: boolean;
        description?: string;
    }
    interface ProviderInfo {
        id: string;
        name: string;
        hasKey: boolean;
        available: boolean; // Changed from isAvailable to match Go backend
        models: ModelInfo[];
    }
    const [providers, setProviders] = useState<ProviderInfo[]>([]);

    // Fallback static models if providers not loaded
    const FALLBACK_MODELS: Record<string, ModelInfo[]> = {
        gemini: [{ id: 'gemini-3-flash', name: 'Gemini 3 Flash', contextWindow: 1000000, inputPrice: 0, outputPrice: 0, isFree: true, supportsTools: true }],
        anthropic: [{ id: 'claude-sonnet-4-20250514', name: 'Claude Sonnet 4', contextWindow: 200000, inputPrice: 3, outputPrice: 15, isFree: false, supportsTools: true }],
        openai: [{ id: 'gpt-4o', name: 'GPT-4o', contextWindow: 128000, inputPrice: 2.5, outputPrice: 10, isFree: false, supportsTools: true }],
        xai: [{ id: 'grok-code-fast-1', name: 'Grok Code Fast', contextWindow: 128000, inputPrice: 0.15, outputPrice: 0.6, isFree: false, supportsTools: true }],
        deepseek: [{ id: 'deepseek-chat', name: 'DeepSeek V3.2', contextWindow: 128000, inputPrice: 0.27, outputPrice: 1.10, isFree: false, supportsTools: true }],
        minimax: [{ id: 'MiniMax-M2.1', name: 'MiniMax M2.1', contextWindow: 200000, inputPrice: 0.5, outputPrice: 2, isFree: false, supportsTools: true }],
        openrouter: [{ id: 'anthropic/claude-sonnet-4', name: 'Claude Sonnet 4', contextWindow: 200000, inputPrice: 3, outputPrice: 15, isFree: false, supportsTools: true }],
    };

    // Get current provider's models
    const currentProvider = providers.find(p => p.id === provider);
    const availableModels = currentProvider?.models?.length ? currentProvider.models : (FALLBACK_MODELS[provider] || []);

    // Reset model when provider changes
    useEffect(() => {
        const models = currentProvider?.models?.length ? currentProvider.models : (FALLBACK_MODELS[provider] || []);
        if (models.length > 0 && (!model || !models.find(m => m.id === model))) {
            setModel(models[0].id);
        }
    }, [provider, currentProvider]);

    // Live Mode State
    const [telegramToken, setTelegramToken] = useState('');
    const [telegramChatId, setTelegramChatId] = useState('');
    const [botInfo, setBotInfo] = useState<BotInfo | null>(null);
    const [isVerifying, setIsVerifying] = useState(false);
    const [testStatus, setTestStatus] = useState<'idle' | 'sending' | 'success' | 'error'>('idle');

    // Automation & Guardrails State
    const [autoApproval, setAutoApproval] = useState<AutoApprovalSettings>({
        enabled: true,
        read_files: true,
        read_files_external: false,
        edit_files: false,
        edit_files_external: false,
        execute_safe_commands: true,
        execute_all_commands: false,
        use_browser: false,
        use_mcp: true,
    });

    const [contextSettings, setContextSettings] = useState<ContextSettings>({
        auto_condense: true,
        condense_threshold: 70,
        sliding_window_size: 20,
        show_context_indicator: true,
        enable_checkpoints: true,
        checkpoint_on_writes: true,
    });

    const [soundEnabled, setSoundEnabled] = useState(false);

    const { postMessage, onMessage } = useVSCodeApi();


    // Load settings and models from backend on mount
    useEffect(() => {
        postMessage({ type: 'get_settings' });
        postMessage({ type: 'get_models' });
    }, [postMessage]);

    // Listen for settings loaded and bot verification from extension
    useEffect(() => {
        const unsubscribe = onMessage((message) => {
            if (message.type === 'settings_loaded') {
                const s = message.payload as Record<string, unknown>;
                if (s.provider) setProvider(s.provider as string);
                if (s.model) setModel(s.model as string);
                if (s.apiKeys) setApiKeys(s.apiKeys as Record<string, string>);
                if (s.telegramToken) setTelegramToken(s.telegramToken as string);
                if (s.telegramChatId) setTelegramChatId(String(s.telegramChatId));
                if (s.context) setContextSettings(s.context as ContextSettings);
                if (s.auto_approval) setAutoApproval(s.auto_approval as AutoApprovalSettings);
                if (s.soundEnabled !== undefined) setSoundEnabled(s.soundEnabled as boolean);
            }
            if (message.type === 'models') {
                const result = message.payload as { providers: ProviderInfo[] };
                // Strictly filter to DeepSeek as requested by the USER
                const filtered = result.providers.filter(p => p.id === 'deepseek');
                setProviders(filtered);
            }
            if (message.type === 'bot_verification_result') {
                const result = message.payload as BotInfo;
                setBotInfo(result);
                setIsVerifying(false);
            }
            if (message.type === 'test_telegram_result') {
                const result = message.payload as { ok: boolean };
                setTestStatus(result.ok ? 'success' : 'error');
            }
        });
        return () => { unsubscribe(); };
    }, [onMessage]);

    // Verify token when it changes (debounced, via extension)
    useEffect(() => {
        if (!telegramToken || telegramToken.length < 20) {
            setBotInfo(null);
            return;
        }

        const timeout = setTimeout(() => {
            setIsVerifying(true);
            postMessage({
                type: 'verify_telegram_token',
                payload: { token: telegramToken }
            });
        }, 500);

        return () => clearTimeout(timeout);
    }, [telegramToken, postMessage]);

    const handleSave = () => {
        postMessage({
            type: 'save_settings',
            payload: {
                apiKeys,
                provider,
                model,
                telegramToken,
                telegramChatId: telegramChatId ? parseInt(telegramChatId, 10) : 0,
                context: contextSettings,
                auto_approval: autoApproval,
                soundEnabled
            }
        });
        onClose();
    };

    return (
        <div className="flex flex-col h-full bg-[#1e1e1e]">
            {/* Header */}
            <div className="flex items-center justify-between px-4 py-3 border-b border-[#333]">
                <div className="flex items-center gap-2">
                    <button
                        onClick={onClose}
                        className="p-1 text-[#888] hover:text-[#ccc] hover:bg-[#2a2d2e] rounded transition-colors"
                        title="Back"
                    >
                        <ChevronLeft className="w-4 h-4" />
                    </button>
                    <h2 className="text-sm font-medium text-[#ccc]">Settings</h2>
                </div>
                <button
                    onClick={handleSave}
                    className="px-4 py-1.5 text-sm font-medium bg-[#0e639c] text-white rounded hover:bg-[#1177bb] transition-colors"
                >
                    Save
                </button>
            </div>

            {/* Content with tabs */}
            <div className="flex flex-1 overflow-hidden">
                {/* Tab sidebar */}
                <div className="w-36 border-r border-[#333] flex flex-col overflow-y-auto">
                    {TABS.map(({ id, label, icon: Icon }) => (
                        <button
                            key={id}
                            onClick={() => setActiveTab(id)}
                            className={`
                                flex items-center gap-2 px-3 py-2.5 text-sm text-left
                                border-l-2 transition-colors
                                ${activeTab === id
                                    ? 'border-[#0e639c] bg-[#04395e]/30 text-[#ccc]'
                                    : 'border-transparent text-[#888] hover:bg-[#2a2d2e] hover:text-[#ccc]'
                                }
                            `}
                        >
                            <Icon className="w-4 h-4" />
                            <span>{label}</span>
                        </button>
                    ))}
                </div>

                {/* Tab content */}
                <div className="flex-1 overflow-y-auto p-4">
                    {activeTab === 'api' && (
                        <div className="space-y-6">
                            {/* Active Provider & Model Selection */}
                            <section className="space-y-4">
                                <h3 className="text-xs font-medium text-[#888] uppercase tracking-wide">
                                    Active Configuration
                                </h3>

                                <div className="space-y-2">
                                    <label className="text-xs text-[#888]">Provider</label>
                                    <Select value={provider} onValueChange={(v) => setProvider(v as typeof provider)}>
                                        <SelectTrigger>
                                            <SelectValue placeholder="Select provider" />
                                        </SelectTrigger>
                                        <SelectContent>
                                            {providers.map((p) => (
                                                <SelectItem key={p.id} value={p.id}>
                                                    <div className="flex items-center gap-2">
                                                        <span className={`w-1.5 h-1.5 rounded-full ${p.hasKey ? 'bg-green-500' : 'bg-yellow-500'}`} />
                                                        {p.name}
                                                    </div>
                                                </SelectItem>
                                            ))}
                                        </SelectContent>
                                    </Select>
                                </div>

                                <div className="space-y-2">
                                    <label className="text-xs text-[#888]">Model</label>
                                    {availableModels.length > 0 ? (
                                        <Select value={model} onValueChange={setModel}>
                                            <SelectTrigger>
                                                <SelectValue placeholder="Select model" />
                                            </SelectTrigger>
                                            <SelectContent>
                                                {availableModels.map((m) => (
                                                    <SelectItem key={m.id} value={m.id}>
                                                        <div className="flex items-center justify-between w-full">
                                                            <span>{m.name}</span>
                                                            <span className="text-[10px] text-[#666] ml-2">
                                                                {m.isFree ? '(free)' : `$${m.inputPrice}/${m.outputPrice}`}
                                                            </span>
                                                        </div>
                                                    </SelectItem>
                                                ))}
                                            </SelectContent>
                                        </Select>
                                    ) : (
                                        <Input
                                            type="text"
                                            value={model}
                                            onChange={(e) => setModel(e.target.value)}
                                            placeholder="deepseek-chat, gpt-4o, claude-sonnet-4..."
                                        />
                                    )}
                                </div>
                            </section>

                            {/* Per-Provider API Keys */}
                            <section className="space-y-4">
                                <h3 className="text-xs font-medium text-[#888] uppercase tracking-wide">
                                    API Keys
                                </h3>
                                <p className="text-[10px] text-[#666]">
                                    Enter your API keys for each provider you want to use
                                </p>

                                <div className="space-y-3">
                                    {providers.map((p) => (
                                        <div key={p.id} className="bg-[#252526] rounded-md p-3 space-y-2">
                                            <div className="flex items-center justify-between">
                                                <div className="flex items-center gap-2">
                                                    <span className={`w-2 h-2 rounded-full ${p.hasKey ? 'bg-green-500' : 'bg-[#444]'}`} />
                                                    <span className="text-sm text-[#ccc] font-medium">{p.name}</span>
                                                </div>
                                                {p.hasKey && (
                                                    <span className="text-[10px] text-green-400 bg-green-400/10 px-2 py-0.5 rounded">
                                                        Connected
                                                    </span>
                                                )}
                                            </div>
                                            <Input
                                                type="password"
                                                value={apiKeys[p.id] || ''}
                                                onChange={(e) => setApiKeys(prev => ({ ...prev, [p.id]: e.target.value }))}
                                                placeholder={`Enter ${p.name} API key...`}
                                                className="text-xs"
                                            />
                                        </div>
                                    ))}
                                </div>
                            </section>
                        </div>
                    )}

                    {activeTab === 'automations' && (
                        <div className="space-y-6 text-[#ccc]">
                            <section className="space-y-4">
                                <h3 className="text-xs font-medium text-[#888] uppercase tracking-wide">
                                    Auto-Approval Guardrails
                                </h3>
                                <div className="space-y-3">
                                    <div className="flex items-start gap-3">
                                        <input
                                            type="checkbox"
                                            id="aa-enabled"
                                            checked={autoApproval.enabled}
                                            onChange={(e) => setAutoApproval(prev => ({ ...prev, enabled: e.target.checked }))}
                                            className="mt-1 accent-[#0e639c]"
                                        />
                                        <div>
                                            <label htmlFor="aa-enabled" className="text-sm font-medium block cursor-pointer">
                                                Enable Auto-Approval
                                            </label>
                                            <p className="text-xs text-[#888] mt-0.5">
                                                Allow the agent to execute actions without confirmation.
                                            </p>
                                        </div>
                                    </div>

                                    {autoApproval.enabled && (
                                        <div className="ml-6 space-y-3 pt-2 border-l border-[#333] pl-4">
                                            {[
                                                { id: 'read_files', label: 'Read Workspace Files' },
                                                { id: 'read_files_external', label: 'Read External Files' },
                                                { id: 'edit_files', label: 'Edit Workspace Files' },
                                                { id: 'execute_safe_commands', label: 'Run Safe Commands' },
                                                { id: 'use_mcp', label: 'Use MCP Tools' },
                                            ].map(opt => (
                                                <div key={opt.id} className="flex items-start gap-3">
                                                    <input
                                                        type="checkbox"
                                                        id={`aa-${opt.id}`}
                                                        checked={(autoApproval as any)[opt.id]}
                                                        onChange={(e) => setAutoApproval(prev => ({ ...prev, [opt.id]: e.target.checked }))}
                                                        className="mt-1 accent-[#0e639c]"
                                                    />
                                                    <div>
                                                        <label htmlFor={`aa-${opt.id}`} className="text-sm block cursor-pointer">
                                                            {opt.label}
                                                        </label>
                                                    </div>
                                                </div>
                                            ))}
                                        </div>
                                    )}
                                </div>
                            </section>

                            <section className="space-y-4 pt-4 border-t border-[#333]">
                                <h3 className="text-xs font-medium text-[#888] uppercase tracking-wide">
                                    Context Management
                                </h3>
                                <div className="space-y-4">
                                    <div className="flex items-start gap-3">
                                        <input
                                            type="checkbox"
                                            id="ctx-condense"
                                            checked={contextSettings.auto_condense}
                                            onChange={(e) => setContextSettings(prev => ({ ...prev, auto_condense: e.target.checked }))}
                                            className="mt-1 accent-[#0e639c]"
                                        />
                                        <div>
                                            <label htmlFor="ctx-condense" className="text-sm font-medium block cursor-pointer">
                                                Auto-Condense Context
                                            </label>
                                            <p className="text-xs text-[#888] mt-0.5">
                                                Automatically summarize old messages when window is nearly full.
                                            </p>
                                        </div>
                                    </div>

                                    <div className="space-y-2 ml-7">
                                        <div className="flex justify-between text-[10px] text-[#888]">
                                            <span>Condense Threshold</span>
                                            <span>{contextSettings.condense_threshold}%</span>
                                        </div>
                                        <input
                                            type="range"
                                            min="50"
                                            max="95"
                                            step="5"
                                            value={contextSettings.condense_threshold}
                                            onChange={(e) => setContextSettings(prev => ({ ...prev, condense_threshold: parseInt(e.target.value, 10) }))}
                                            className="w-full"
                                        />
                                    </div>

                                    <div className="flex items-start gap-3">
                                        <input
                                            type="checkbox"
                                            id="ctx-checkpoints"
                                            checked={contextSettings.enable_checkpoints}
                                            onChange={(e) => setContextSettings(prev => ({ ...prev, enable_checkpoints: e.target.checked }))}
                                            className="mt-1 accent-[#0e639c]"
                                        />
                                        <div>
                                            <label htmlFor="ctx-checkpoints" className="text-sm font-medium block cursor-pointer">
                                                Workspace Checkpoints
                                            </label>
                                        </div>
                                    </div>

                                    {contextSettings.enable_checkpoints && (
                                        <div className="ml-7 flex items-start gap-3">
                                            <input
                                                type="checkbox"
                                                id="ctx-save-on-write"
                                                checked={contextSettings.checkpoint_on_writes}
                                                onChange={(e) => setContextSettings(prev => ({ ...prev, checkpoint_on_writes: e.target.checked }))}
                                                className="mt-1 accent-[#0e639c]"
                                            />
                                            <label htmlFor="ctx-save-on-write" className="text-xs text-[#888] block cursor-pointer">
                                                Auto-save after file edit
                                            </label>
                                        </div>
                                    )}
                                </div>
                            </section>

                            <section className="space-y-4 pt-4 border-t border-[#333]">
                                <h3 className="text-xs font-medium text-[#888] uppercase tracking-wide">
                                    Sounds
                                </h3>
                                <div className="flex items-start gap-3">
                                    <input
                                        type="checkbox"
                                        id="sf-sound"
                                        checked={soundEnabled}
                                        onChange={(e) => setSoundEnabled(e.target.checked)}
                                        className="mt-1 accent-[#0e639c]"
                                    />
                                    <label htmlFor="sf-sound" className="text-sm font-medium block cursor-pointer">
                                        Sound Effects
                                    </label>
                                </div>
                            </section>
                        </div>
                    )}

                    {activeTab === 'live' && (
                        <div className="space-y-6">
                            <section className="space-y-4">
                                <h3 className="text-xs font-medium text-[#888] uppercase tracking-wide">
                                    Ether
                                </h3>

                                <div className="p-3 bg-[#252526] rounded-md">
                                    <div className="flex items-start gap-2">
                                        <Info className="w-4 h-4 text-[#888] flex-shrink-0 mt-0.5" />
                                        <p className="text-xs text-[#888]">
                                            Ether lets you control Ricochet from any messenger
                                            when you're away from your computer.
                                        </p>
                                    </div>
                                </div>

                                <div className="space-y-2">
                                    <label className="text-xs text-[#888]">Telegram Bot Token</label>
                                    <Input
                                        type="password"
                                        value={telegramToken}
                                        onChange={(e) => setTelegramToken(e.target.value)}
                                        placeholder="123456789:ABCdef..."
                                    />

                                    {/* Bot verification status */}
                                    {isVerifying && (
                                        <div className="flex items-center gap-2 text-xs text-blue-400">
                                            <Loader2 className="w-3 h-3 animate-spin" />
                                            <span>Verifying token...</span>
                                        </div>
                                    )}
                                    {botInfo && !isVerifying && (
                                        <div className={`flex items-center gap-2 text-xs ${botInfo.ok ? 'text-green-400' : 'text-red-400'}`}>
                                            {botInfo.ok ? (
                                                <>
                                                    <CheckCircle className="w-3 h-3" />
                                                    <span>Connected: @{botInfo.username} ({botInfo.firstName})</span>
                                                </>
                                            ) : (
                                                <>
                                                    <XCircle className="w-3 h-3" />
                                                    <span>{botInfo.error}</span>
                                                </>
                                            )}
                                        </div>
                                    )}
                                    {!botInfo && !isVerifying && (
                                        <p className="text-xs text-[#666]">
                                            Create a bot with @BotFather on Telegram
                                        </p>
                                    )}
                                </div>

                                <div className="space-y-2">
                                    <label className="text-xs text-[#888]">Your Telegram Chat ID</label>
                                    <div className="flex gap-2">
                                        <Input
                                            type="text"
                                            value={telegramChatId}
                                            onChange={(e) => setTelegramChatId(e.target.value)}
                                            placeholder="123456789"
                                            className="flex-1"
                                        />
                                        <button
                                            onClick={() => {
                                                if (!telegramToken || !telegramChatId) return;
                                                setTestStatus('sending');
                                                postMessage({
                                                    type: 'test_telegram',
                                                    payload: { token: telegramToken, chatId: parseInt(telegramChatId, 10) }
                                                });
                                            }}
                                            disabled={!telegramToken || !telegramChatId || testStatus === 'sending'}
                                            className="px-3 py-1.5 text-xs font-medium bg-[#0e639c] text-white rounded hover:bg-[#1177bb] disabled:opacity-50 disabled:cursor-not-allowed transition-colors whitespace-nowrap"
                                        >
                                            {testStatus === 'sending' ? '...' : 'Send Test'}
                                        </button>
                                    </div>
                                    {testStatus === 'success' && (
                                        <div className="flex items-center gap-2 text-xs text-green-400">
                                            <CheckCircle className="w-3 h-3" />
                                            <span>Test message sent!</span>
                                        </div>
                                    )}
                                    {testStatus === 'error' && (
                                        <div className="flex items-center gap-2 text-xs text-red-400">
                                            <XCircle className="w-3 h-3" />
                                            <span>Failed to send. Check Chat ID.</span>
                                        </div>
                                    )}
                                    {!testStatus && (
                                        <p className="text-xs text-[#666]">
                                            Get your Chat ID by messaging @userinfobot
                                        </p>
                                    )}
                                </div>
                            </section>
                        </div>
                    )}

                    {activeTab === 'about' && (
                        <div className="space-y-6">
                            <section className="space-y-4">
                                <div className="flex items-center gap-4">
                                    <div className="w-16 h-16 bg-gradient-to-br from-[#0e639c] to-[#1177bb] rounded-xl flex items-center justify-center shadow-lg">
                                        <RicochetLogo className="w-8 h-8 text-white" />
                                    </div>
                                    <div>
                                        <h3 className="text-lg font-bold text-white tracking-tight">Ricochet</h3>
                                        <p className="text-sm text-[#888]">Version 0.0.1</p>
                                        <div className="flex gap-2 mt-2">
                                            <a href="https://github.com/Grik-ai/ricochet" target="_blank" rel="noreferrer" className="text-[#888] hover:text-white transition-colors">
                                                <Github className="w-4 h-4" />
                                            </a>
                                            <a href="https://www.linkedin.com/in/igoryan34/" target="_blank" rel="noreferrer" className="text-[#888] hover:text-white transition-colors">
                                                <Linkedin className="w-4 h-4" />
                                            </a>
                                            <a href="https://x.com/genecental" target="_blank" rel="noreferrer" className="text-[#888] hover:text-white transition-colors">
                                                <Twitter className="w-4 h-4" />
                                            </a>
                                        </div>
                                    </div>
                                </div>

                                <div className="space-y-4 pt-2">
                                    <p className="text-sm text-[#ccc] leading-relaxed">
                                        Building <strong>Ricochet</strong> — the first hybrid AI agent and the foundation for a future decentralized ecosystem (<strong>GameFi, Crowdfunding, DAO</strong>).
                                    </p>
                                    <p className="text-sm text-[#ccc] leading-relaxed">
                                        I'm an independent engineer building tools that empower creators. Your support helps me:
                                    </p>
                                    <ul className="text-sm text-[#ccc] list-disc list-inside space-y-1 ml-1">
                                        <li>Evolve the AI agent (JetBrains support, smarter autonomy).</li>
                                        <li>Build the <strong>Gaming & Crowdfunding</strong> platform.</li>
                                        <li>Keep the core open-source and free.</li>
                                    </ul>
                                </div>
                            </section>

                            <section className="space-y-3 pt-4 border-t border-[#333]">
                                <h3 className="text-xs font-medium text-[#888] uppercase tracking-wide flex items-center gap-2">
                                    <Heart className="w-3 h-3 text-red-400" />
                                    Support the Project
                                </h3>

                                <div className="grid grid-cols-2 gap-3">
                                    <a
                                        href="https://ko-fi.com/igoryan34"
                                        target="_blank"
                                        rel="noreferrer"
                                        className="flex items-center justify-center gap-2 p-3 bg-[#29abe0]/10 hover:bg-[#29abe0]/20 text-[#29abe0] rounded-lg border border-[#29abe0]/20 transition-all hover:scale-[1.02]"
                                    >
                                        <Coffee className="w-4 h-4" />
                                        <span className="text-sm font-medium">Buy me a coffee</span>
                                    </a>
                                    <a
                                        href="https://www.paypal.com/ncp/payment/PPMFBMFVAB8QN"
                                        target="_blank"
                                        rel="noreferrer"
                                        className="flex items-center justify-center gap-2 p-3 bg-[#0070ba]/10 hover:bg-[#0070ba]/20 text-[#0070ba] rounded-lg border border-[#0070ba]/20 transition-all hover:scale-[1.02]"
                                    >
                                        <CreditCard className="w-4 h-4" />
                                        <span className="text-sm font-medium">PayPal</span>
                                    </a>
                                </div>

                                <div className="space-y-2 mt-2">
                                    <p className="text-xs text-[#666] mb-2">Crypto Wallets (Click to copy)</p>
                                    {[
                                        { label: 'TON', value: 'UQB93GTsF6ZI7ljBViLr-IHIf93HpqwolC51jR5Und7GAwm4' },
                                        { label: 'USDT (TRC20)', value: 'TH1ZvpbmNKtArQ2zNyoeAq4zvU3koNTFhj' },
                                        { label: 'EVM (BEP20)', value: '0x048911b8690cd7c85a0898dffbd5e3b9ba50dd10' },
                                        { label: 'Bitcoin', value: '13fC3C2yRq4i8meaUqHWK6H5UQ2V1Bk8Ct' }
                                    ].map((wallet) => (
                                        <div
                                            key={wallet.label}
                                            onClick={() => {
                                                navigator.clipboard.writeText(wallet.value);
                                                // Could add toast here, but for now simple copy
                                            }}
                                            className="group flex items-center justify-between p-2 bg-[#252526] hover:bg-[#2a2d2e] rounded border border-[#333] cursor-pointer transition-colors"
                                            title="Click to copy"
                                        >
                                            <div className="flex flex-col overflow-hidden">
                                                <span className="text-[10px] text-[#888] font-medium">{wallet.label}</span>
                                                <span className="text-xs text-[#ccc] font-mono truncate">{wallet.value}</span>
                                            </div>
                                            <Copy className="w-3 h-3 text-[#666] group-hover:text-white transition-colors" />
                                        </div>
                                    ))}
                                </div>
                            </section>

                            <div className="pt-4 border-t border-[#333]">
                                <p className="text-xs text-[#666] text-center">
                                    Built with ❤️ by the Ricochet team
                                </p>
                            </div>
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
}
