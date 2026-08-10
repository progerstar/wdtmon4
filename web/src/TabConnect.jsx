import { useEffect, useRef, useState } from "react";
import axios from 'axios';
import { AiOutlineClose, AiOutlineEye, AiOutlineEyeInvisible } from "react-icons/ai";
import I18n from './I18n';
import { safeDashboardUrl } from './cloudToken';

const CLOUD_URL = "https://cloud.unitx.pro/";
const CLOUD_REQUEST_TIMEOUT = 5000;

function hiddenToken(value) {
    return String(value || "").startsWith('utx1_') ? 'utx1_••••••••••••' : '••••••••••••';
}

function TokenValue({label, value, revealed}) {
    return (
        <div className="min-w-0 rounded-xl bg-base-200/70 p-3">
            <dt className="text-xs font-medium uppercase tracking-wide text-base-content/60">{label}</dt>
            <dd className="mt-1 min-w-0 break-all font-mono text-sm" translate="no">
                {revealed ? value : hiddenToken(value)}
            </dd>
        </div>
    );
}

export default function TabConnect(props) {
    const { settings, setSettings } = props;
    const [tokensVisible, setTokensVisible] = useState(false);
    const [clearPending, setClearPending] = useState(false);
    const [clearError, setClearError] = useState("");
    const clearControllerRef = useRef(null);
    const writeToken = settings.ConWriteToken || settings.ConUID || "";
    const readToken = settings.ConReadToken || "";
    const dashboardUrl = safeDashboardUrl(settings.ConDashboardURL)
        || (readToken ? safeDashboardUrl(`${CLOUD_URL}#read_token=${encodeURIComponent(readToken)}`) : "");
    const hasEphemeralTokens = Boolean(writeToken || readToken);

    useEffect(() => () => clearControllerRef.current?.abort(), []);

    const clear = async () => {
        if (!window.confirm(I18n.get('Clear tokens confirmation'))) return;
        const controller = new AbortController();
        clearControllerRef.current = controller;
        setClearPending(true);
        setClearError("");
        try {
            await axios.post('/con/clear', undefined, {
                timeout: CLOUD_REQUEST_TIMEOUT,
                signal: controller.signal,
            });
            if (controller.signal.aborted) return;
            setSettings({
                ConEn: false,
                ConUID: "",
                ConReadToken: "",
                ConWriteToken: "",
                ConDashboardURL: "",
                ConConfigured: false,
            });
        } catch {
            if (!controller.signal.aborted) setClearError(I18n.get('Failed to clear tokens'));
        } finally {
            if (clearControllerRef.current === controller) clearControllerRef.current = null;
            if (!controller.signal.aborted) setClearPending(false);
        }
    };

    return (
        <div className="space-y-4">
            {clearError && <div role="alert" aria-live="polite" className="alert alert-error break-words">{clearError}</div>}
            <section aria-labelledby="cloud-connection-title" className="card w-full rounded-2xl border border-base-300/70 bg-base-100 p-4 shadow-lg sm:p-5">
                <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                    <h2 id="cloud-connection-title" className="text-lg font-semibold">{I18n.get('Cloud connection')}</h2>
                    <div className="flex flex-wrap items-center gap-2">
                        {hasEphemeralTokens && <button type="button" aria-pressed={tokensVisible} onClick={()=>setTokensVisible((visible) => !visible)} className="btn btn-ghost min-h-11 gap-2 sm:btn-sm sm:min-h-0">
                            {tokensVisible ? <AiOutlineEyeInvisible aria-hidden="true"/> : <AiOutlineEye aria-hidden="true"/>}
                            {tokensVisible ? I18n.get('Hide tokens') : I18n.get('Show tokens')}
                        </button>}
                        {dashboardUrl && <a href={dashboardUrl} target="_blank" rel="noreferrer" className="btn btn-ghost min-h-11 sm:btn-sm sm:min-h-0">{I18n.get('Dashboard')}</a>}
                        <button type="button" aria-label={clearPending ? I18n.get('Clearing…') : I18n.get('Clear tokens')} title={I18n.get('Clear tokens')} onClick={()=>{void clear()}} className="btn btn-ghost btn-square min-h-11 text-error sm:btn-sm sm:min-h-0" disabled={clearPending}>
                            <AiOutlineClose aria-hidden="true"/>
                        </button>
                    </div>
                </div>

                <dl className="mt-4 grid min-w-0 gap-3 lg:grid-cols-2">
                    {writeToken
                        ? <TokenValue label={I18n.get('Write token')} value={writeToken} revealed={tokensVisible}/>
                        : <div className="min-w-0 rounded-xl bg-base-200/70 p-3">
                            <dt className="text-xs font-medium uppercase tracking-wide text-base-content/60">{I18n.get('Write token')}</dt>
                            <dd className="mt-1 text-sm">{I18n.get('Write token is configured.')}</dd>
                        </div>}
                    {readToken && <TokenValue label={I18n.get('Read token')} value={readToken} revealed={tokensVisible}/>}
                </dl>

                {hasEphemeralTokens && <p className="mt-3 text-pretty text-sm text-base-content/70">
                    {readToken ? I18n.get('Save these tokens. The read token is shown once and is not saved in settings.json.') : I18n.get('Save this write token.')}
                </p>}
            </section>

            <section aria-label={I18n.get('Device ID')} className="card w-full rounded-2xl border border-base-300/70 bg-base-100 p-4 shadow-lg sm:p-5">
                <div className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_minmax(12rem,20rem)] sm:items-center sm:gap-4">
                    <label htmlFor="device-id" className="label-text font-medium">{I18n.get('Device ID')}</label>
                    <input id="device-id" type="text" name="device-id" autoComplete="off" spellCheck={false} value={settings.ConDev || ""} onChange={(e)=>{ setSettings({ConDev: e.target.value})} } className="input input-bordered input-accent w-full min-w-0 sm:input-sm" />
                </div>
            </section>
        </div>
    );
}
