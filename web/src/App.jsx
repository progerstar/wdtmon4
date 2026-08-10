import { useCallback, useEffect, useRef, useState } from "react";
import TabMain from './TabMain';
import TabSettings from './TabSettings';
import TabConnect from './TabConnect';
import axios from 'axios';
import I18n from './I18n';
import Login from './Login';
import { isCloudWriteToken } from './cloudToken';

const SETTINGS_REQUEST_TIMEOUT = 5000;
const initialSettings = {
  ConEn: false, Net: "", NetEn: false, Proc: "", ProcEn: false,
  Diode: true, Pause: false, ConUID: "", ConReadToken: "", ConWriteToken: "",
  ConDashboardURL: "", ConDev: "", ConConfigured: false, Empty: true,
};

function normalizeLoadedSettings(data) {
  if (!data || typeof data !== "object" || Array.isArray(data)) {
    throw new Error("invalid settings response");
  }
  return {
    ...initialSettings,
    ...data,
    ConUID: "",
    ConReadToken: "",
    ConWriteToken: "",
    ConDashboardURL: "",
    Empty: false,
  };
}

function settingsForSave(settings) {
  const {
    ConReadToken: _readToken,
    ConDashboardURL: _dashboardUrl,
    ConWriteToken: _writeToken,
    ConUID: _legacyToken,
    ConConfigured: _configured,
    Empty: _empty,
    ...persistedSettings
  } = settings;
  return persistedSettings;
}

export default function App () {
  const [ settings, setSettings ] = useState(initialSettings);

  const [showSettings, setShowSettings] = useState(false);
  const [present, setPresent] = useState(false);
  const [settingsError, setSettingsError] = useState("");
  const settingsRef = useRef(initialSettings);
  const lastSavedRef = useRef(initialSettings);
  const saveInFlight = useRef(false);
  const saveQueued = useRef(false);
  const saveControllerRef = useRef(null);
  const mountedRef = useRef(true);

  const queueSettingsSave = useCallback(() => {
    if (settingsRef.current.Empty) return;

    saveQueued.current = true;
    if (saveInFlight.current) return;

    const save = async () => {
      saveInFlight.current = true;
      try {
        while (saveQueued.current && mountedRef.current) {
          saveQueued.current = false;
          const snapshot = settingsRef.current;
          const controller = new AbortController();
          saveControllerRef.current = controller;
          try {
            await axios.post('/settings', settingsForSave(snapshot), {
              timeout: SETTINGS_REQUEST_TIMEOUT,
              signal: controller.signal,
            });
            lastSavedRef.current = snapshot;
            if (mountedRef.current) setSettingsError("");
          } catch (error) {
            if (controller.signal.aborted || !mountedRef.current) return;

            const status = error.response?.status;
            const permanentFailure = status >= 400 && status < 500;
            const hasNewerSnapshot = settingsRef.current !== snapshot || saveQueued.current;
            if (permanentFailure && !hasNewerSnapshot) {
              const rollback = lastSavedRef.current;
              settingsRef.current = rollback;
              setSettings(rollback);
            }
            setSettingsError(I18n.get('Failed to save settings'));
          } finally {
            if (saveControllerRef.current === controller) saveControllerRef.current = null;
          }
        }
      } finally {
        saveInFlight.current = false;
      }
    };

    void save();
  }, []);

  const updateSettings = useCallback((val) => {
    const current = settingsRef.current;
    if (!current || current.Empty) return;

    const next = { ...current, ...val };
    settingsRef.current = next;
    setSettings(next);
    queueSettingsSave();
  }, [queueSettingsSave]);

  useEffect(() => {
    const controller = new AbortController();
    axios.get('/settings', {
      timeout: SETTINGS_REQUEST_TIMEOUT,
      signal: controller.signal,
    })
      .then((res) => {
        if (controller.signal.aborted) return;
        const loaded = normalizeLoadedSettings(res.data);
        settingsRef.current = loaded;
        lastSavedRef.current = loaded;
        setSettings(loaded);
        setSettingsError("");
      })
      .catch(() => {
        if (controller.signal.aborted) return;
        setSettingsError(I18n.get('Failed to load settings'));
      });
    return () => controller.abort();
  }, []);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      saveControllerRef.current?.abort();
    };
  }, []);

  useEffect(() => {
    if (!present) setShowSettings(false);
  }, [present]);

  const cloudConfigured = Boolean(
    settings.ConConfigured || isCloudWriteToken(settings.ConWriteToken || settings.ConUID),
  );

  return ( <div className="min-h-screen w-full overflow-x-hidden bg-base-200 bg-repeat p-3 sm:p-6 lg:p-8" style={{backgroundImage: "radial-gradient(hsla(var(--bc) /.2) .5px, hsla(var(--b2) /1) .5px)", backgroundSize: "5px 5px"}}>
              <a className="skip-link" href="#main-content">{I18n.get('Skip to main content')}</a>
              <main id="main-content" tabIndex={-1} className="mx-auto min-h-full w-full max-w-5xl">
                <h1 className="divider mt-0 text-base font-medium sm:text-lg">{I18n.get('Main')}</h1>
                {settingsError && <div role="alert" aria-live="polite" className="alert alert-error mb-4 break-words">{settingsError}</div>}
                <TabMain settings={settings} setSettings={updateSettings} setPresentExt={setPresent}/>
                <section aria-label={I18n.get('Application controls')} className="my-6 flex min-h-14 flex-wrap items-center justify-center gap-x-5 gap-y-1 border-y border-base-300 py-1 sm:my-8">
                        <label className="inline-flex min-h-11 cursor-pointer items-center gap-2">
                          <span>{I18n.get('Settings')}</span>
                          <input aria-label={I18n.get('Settings')} checked={present && showSettings} onChange={()=>{setShowSettings((visible) => !visible)}} type="checkbox" className="toggle sm:toggle-sm" disabled={!present}/>
                        </label>
                        <label className="inline-flex min-h-11 cursor-pointer items-center gap-2">
                          <span>{I18n.get('Cloud')}</span>
                          <input aria-label={I18n.get('Cloud')} checked={Boolean(settings.ConEn)} onChange={()=>{updateSettings({ConEn: !settings.ConEn})}} type="checkbox" className="toggle sm:toggle-sm" disabled={settings.Empty}/>
                        </label>
                    </section>
                  { present && showSettings && <TabSettings/> }
                  { settings.ConEn &&
                    <>
                      { cloudConfigured ? <TabConnect settings={settings} setSettings={updateSettings}/> : <Login setSettings={updateSettings}/> }
                    </>
                  }
              </main>
            </div>

  );
}
