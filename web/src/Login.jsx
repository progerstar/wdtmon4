import { useEffect, useRef, useState } from "react";
import axios from 'axios';
import I18n from './I18n';
import { isCloudDeviceId, isCloudWriteToken, safeDashboardUrl } from './cloudToken';

const CLOUD_REQUEST_TIMEOUT = 12000;

export default function Login(props) {
  const { setSettings } = props;
  const inputRef = useRef();
  const [error, setError] = useState("");
  const [pendingAction, setPendingAction] = useState("");
  const requestRef = useRef(null);
  const busy = pendingAction !== "";

  useEffect(() => () => requestRef.current?.abort(), []);

  const beginRequest = () => {
    requestRef.current?.abort();
    const controller = new AbortController();
    requestRef.current = controller;
    return controller;
  };

  const login = async (event) => {
    event.preventDefault();
    if (busy) return;

    const token = inputRef.current?.value.trim() || "";
    if (!isCloudWriteToken(token)) {
      setError(I18n.get('Invalid write token'));
      return;
    }

    setError("");
    setPendingAction("validate");
    const controller = beginRequest();
    try {
      const res = await axios.post('/con/validate', { write_token: token }, {
        timeout: CLOUD_REQUEST_TIMEOUT,
        signal: controller.signal,
      });
      if (controller.signal.aborted) return;
      if (!isCloudDeviceId(res.data?.device_id)) {
        setError(I18n.get('Invalid cloud response'));
        return;
      }
      setSettings({
        ConUID: "",
        ConReadToken: "",
        ConWriteToken: "",
        ConDashboardURL: "",
        ConDev: res.data.device_id,
        ConConfigured: true,
      });
    } catch (requestError) {
      if (controller.signal.aborted) return;
      if ([400, 401, 403].includes(requestError.response?.status)) {
        setError(I18n.get('Invalid write token'));
      } else {
        setError(I18n.get('Cloud service unavailable'));
      }
    } finally {
      if (requestRef.current === controller) requestRef.current = null;
      if (!controller.signal.aborted) setPendingAction("");
    }
  };

  const create = async () => {
    if (busy) return;

    setError("");
    setPendingAction("create");
    const controller = beginRequest();
    try {
      const res = await axios.post('/con/create', undefined, {
        timeout: CLOUD_REQUEST_TIMEOUT,
        signal: controller.signal,
      });
      if (controller.signal.aborted) return;
      if (
        !isCloudWriteToken(res.data?.write_token)
        || !isCloudWriteToken(res.data?.read_token)
        || !isCloudDeviceId(res.data?.device_id)
      ) {
        setError(I18n.get('Invalid cloud response'));
        return;
      }
      setSettings({
        ConUID: res.data.write_token,
        ConReadToken: res.data.read_token,
        ConWriteToken: res.data.write_token,
        ConDashboardURL: safeDashboardUrl(res.data.dashboard_url),
        ConDev: res.data.device_id,
        ConConfigured: true,
      });
    } catch {
      if (controller.signal.aborted) return;
      setError(I18n.get('Cloud service unavailable'));
    } finally {
      if (requestRef.current === controller) requestRef.current = null;
      if (!controller.signal.aborted) setPendingAction("");
    }
  };

  return (
      <section aria-labelledby="cloud-lite-title" className="card w-full rounded-2xl border border-base-300/70 bg-base-100 shadow-lg">
        <div className="grid w-full gap-6 p-4 sm:p-6 lg:grid-cols-[minmax(0,24rem)_minmax(0,1fr)] lg:items-center lg:gap-10 lg:p-10">
          <div className="text-center lg:order-2 lg:text-left">
            <h2 id="cloud-lite-title" className="text-balance text-3xl font-bold sm:text-4xl lg:text-5xl" translate="no">{I18n.get('Cloud Lite')}</h2>
            <p className="pt-3 text-pretty text-base-content/70 sm:pt-4">{I18n.get('Simple and easy to use cloud system')}</p>
          </div>

          <div className="card w-full min-w-0 bg-base-200/60 shadow-inner lg:order-1">
            <form className="card-body gap-4 p-4 sm:p-6" onSubmit={login} aria-busy={busy}>
              <h3 className="card-title text-lg sm:text-xl">{I18n.get('I already have a token')}</h3>
              {error && <p id="cloud-token-error" role="alert" aria-live="polite" className="text-sm text-error">{error}</p>}

              <div className="form-control">
                <label className="label" htmlFor="write-token">
                  <span className="label-text">{I18n.get('Write token')}</span>
                </label>
                <input id="write-token" name="write-token" type="text" ref={inputRef} autoComplete="off" spellCheck={false} aria-invalid={Boolean(error)} aria-describedby={error ? 'cloud-token-error cloud-token-help' : 'cloud-token-help'} placeholder="utx1_…" className="input input-bordered w-full min-w-0" />
                <p id="cloud-token-help" className="mt-2 text-pretty text-xs text-base-content/65">{I18n.get('Only the write token is saved. Keep your read token separately.')}</p>
              </div>

              <button type="submit" className="btn btn-primary w-full" disabled={busy}>
                {pendingAction === "validate" ? I18n.get('Validating…') : I18n.get('Use existing token')}
              </button>

              <div className="divider my-0 text-sm">{I18n.get('or')}</div>

              <button type="button" onClick={()=>{void create()}} className="btn btn-outline w-full" disabled={busy}>
                {pendingAction === "create" ? I18n.get('Creating…') : I18n.get('Get new tokens')}
              </button>
              <p className="text-pretty text-xs text-base-content/65">{I18n.get('New read and write tokens will be shown once. Save them immediately.')}</p>
            </form>
          </div>
        </div>
      </section>
  );
}
