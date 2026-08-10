import { useCallback, useEffect, useRef, useState } from 'react';
import I18n from './I18n';
import axios from 'axios';
import useInterval from './useInterval';
import ProcDialog from './ProcDialog';
import { AiOutlineReload, AiOutlinePoweroff, AiOutlineClose } from "react-icons/ai";

const NODEV = "------------";
const NOTEMP = "--.--";
const SERIAL_REQUEST_TIMEOUT = 3000;
const UPTIME_REQUEST_TIMEOUT = 3000;
const SWITCH_RESPONSE = {
  diode: /^~L[01]$/,
  pause: /^~P[01]$/,
};
const COMMAND_CONFIRMATIONS = {
  '~T1': 'Restart the connected PC now?',
  '~T2': 'Send the Power command to the connected PC now?',
  '~T3': 'Shut down the connected PC now?',
};
const COMMAND_RESPONSES = {
  '~T1': /^~T1$/,
  '~T2': /^~T2$/,
  '~T3': /^~T3$/,
};

const serialCommand = (cmd, signal) => axios.post('/cmd/' + cmd, undefined, {
  timeout: SERIAL_REQUEST_TIMEOUT,
  signal,
});

function formatDate(n) {
  const day = Math.floor(n / (24 * 3600));
  n = n % (24 * 3600);
  var hour = '' + Math.floor(n / 3600);
  n %= 3600;
  var minute =  '' + Math.floor(n / 60) ;
  n %= 60;
  var seconds =  '' + n;

  if (hour.length < 2)  hour = '0' + hour;
  if (minute.length < 2) minute = '0' + minute;
  if (seconds.length < 2) seconds = '0' + seconds;
  return [day+I18n.get("d"), hour+I18n.get("h"), minute+I18n.get("m"), seconds+I18n.get("s")].join(' ');
}

export default function TabMain(props) {
  const { settings, setSettings, setPresentExt } = props;
  const [info, setInfo] = useState(NODEV);
  const [present, setPresent] = useState(false);
  const [commandPending, setCommandPending] = useState(false);
  const [commandFeedback, setCommandFeedback] = useState(null);

  const [uptime, setUptime] = useState(null);
  const [temp, setTemp] = useState(NOTEMP);
  const pollInFlight = useRef(false);
  const uptimeInFlight = useRef(false);
  const uptimeControllerRef = useRef(null);
  const pollControllerRef = useRef(null);
  const commandControllerRef = useRef(null);
  const mountedRef = useRef(true);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      uptimeControllerRef.current?.abort();
      pollControllerRef.current?.abort();
      commandControllerRef.current?.abort();
    };
  }, []);

  useInterval(async () => {
    if (uptimeInFlight.current) return;
    uptimeInFlight.current = true;
    const controller = new AbortController();
    uptimeControllerRef.current = controller;
    try {
      const res = await axios.get('/uptime', {
        timeout: UPTIME_REQUEST_TIMEOUT,
        signal: controller.signal,
      });
      if (mountedRef.current && !controller.signal.aborted) setUptime(formatDate(res.data));
    } catch {
      if (mountedRef.current && !controller.signal.aborted) setUptime(null);
    } finally {
      if (uptimeControllerRef.current === controller) uptimeControllerRef.current = null;
      uptimeInFlight.current = false;
    }
  }, 1000);

  const markDisconnected = useCallback(() => {
    if (!mountedRef.current) return;
    setInfo(NODEV);
    setPresent(false);
    setPresentExt(false);
  }, [setPresentExt]);

  const pollDevice = useCallback(async () => {
    if (pollInFlight.current) return;
    pollInFlight.current = true;
    const controller = new AbortController();
    pollControllerRef.current = controller;

    try {
      let infoResponse;
      try {
        infoResponse = await serialCommand('~I', controller.signal);
      } catch {
        if (controller.signal.aborted || !mountedRef.current) return;
        markDisconnected();
        setTemp(NOTEMP);
        return;
      }

      if (controller.signal.aborted || !mountedRef.current) return;

      const infoData = String(infoResponse.data);
      if (!infoData.startsWith('~I')) {
        markDisconnected();
        setTemp(NOTEMP);
        return;
      }

      setInfo(infoData.slice(2));
      setPresent(true);
      setPresentExt(true);

      try {
        const heartbeatResponse = await serialCommand('~U', controller.signal);
        if (!String(heartbeatResponse.data).startsWith('~A')) {
          setTemp(NOTEMP);
          return;
        }

        const temperatureResponse = await serialCommand('~G', controller.signal);
        if (mountedRef.current && !controller.signal.aborted) setTemp(temperatureResponse.data);
      } catch {
        if (mountedRef.current && !controller.signal.aborted) setTemp(NOTEMP);
      }
    } finally {
      if (pollControllerRef.current === controller) pollControllerRef.current = null;
      pollInFlight.current = false;
    }
  }, [markDisconnected, setPresentExt]);

  useInterval(pollDevice, 5000);

  const runDeviceCommand = useCallback(async (cmd, responsePattern = null) => {
    if (commandControllerRef.current) return null;
    const controller = new AbortController();
    commandControllerRef.current = controller;
    setCommandPending(true);
    try {
      const response = await serialCommand(cmd, controller.signal);
      if (controller.signal.aborted || !mountedRef.current) return null;
      const responseData = String(response.data ?? "");
      if (responseData === "" || (responsePattern && !responsePattern.test(responseData))) {
        throw new Error(`unexpected response to ${cmd}`);
      }
      setCommandFeedback({type: "success", text: I18n.get('Device command sent')});
      return responseData;
    } catch {
      if (controller.signal.aborted || !mountedRef.current) return null;
      setCommandFeedback({type: "error", text: I18n.get('Device command failed')});
      return null;
    } finally {
      if (commandControllerRef.current === controller) commandControllerRef.current = null;
      if (mountedRef.current && !controller.signal.aborted) setCommandPending(false);
    }
  }, []);

  const setDiode = async () => {
    const response = await runDeviceCommand('~L'+(!settings.Diode? "1":"0"), SWITCH_RESPONSE.diode);
    if (response !== null) {
      setSettings({Diode: response[2] === "1"});
    }
  };

  const setPause = async () => {
    const response = await runDeviceCommand('~P'+(!settings.Pause? "1":"0"), SWITCH_RESPONSE.pause);
    if (response !== null) {
      setSettings({Pause: response[2] === "1"});
    }
  };

  const runConfirmedCommand = (cmd) => {
    if (!window.confirm(I18n.get(COMMAND_CONFIRMATIONS[cmd]))) return;
    void runDeviceCommand(cmd, COMMAND_RESPONSES[cmd]);
  };

  const statusText = present ? I18n.get('Device connected') : I18n.get('Device disconnected');

  return (<div>
    <section aria-label={I18n.get('Device status')} className="card min-h-12 w-full rounded-2xl border border-base-300/70 bg-base-100 p-4 shadow-lg sm:flex-row sm:items-center sm:gap-4 sm:px-6 sm:py-4">
      <div className="flex min-w-0 items-center gap-4">
        <p className="truncate font-mono" title={info}>{info}</p>

        <span className="relative inline-flex h-4 w-4 shrink-0" role="status" aria-label={statusText} title={statusText}>
          { present && <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-accent opacity-75"></span> }
          <span className={`relative inline-flex rounded-full h-4 w-4 ${present ? 'bg-accent' : 'bg-base-300 ring-1 ring-base-content/20'}`}></span>
        </span>
      </div>

      <div className="mt-3 flex min-w-0 flex-wrap items-center justify-start gap-x-3 gap-y-2 tabular-nums sm:ml-auto sm:mt-0 sm:flex-nowrap sm:justify-end sm:gap-4">
        <p className="whitespace-nowrap">{uptime ?? '—'}</p>
        <p className="whitespace-nowrap">{temp}°C</p>
        <a href="/monitor" target="_blank" rel="noreferrer" className="link link-accent link-hover">{I18n.get("Monitor")}</a>
        <div className="flex w-full shrink-0 items-center gap-2 pt-1 sm:w-auto sm:gap-1 sm:pt-0">
          <div className="tooltip tooltip-bottom sm:tooltip-left" data-tip={I18n.get('Reset')}>
            <button type="button" aria-label={I18n.get('Reset')} onClick={()=>runConfirmedCommand('~T1')} className='btn btn-outline btn-square base-content sm:btn-sm' disabled={!present || commandPending}><AiOutlineReload aria-hidden="true"/></button>
          </div>
          <div className="tooltip tooltip-bottom sm:tooltip-left" data-tip={I18n.get('Power')}>
            <button type="button" aria-label={I18n.get('Power')} onClick={()=>runConfirmedCommand('~T2')} className='btn btn-outline btn-square base-content sm:btn-sm' disabled={!present || commandPending}><AiOutlinePoweroff aria-hidden="true"/></button>
          </div>
          <div className="tooltip tooltip-bottom sm:tooltip-left" data-tip={I18n.get('Shutdown')}>
            <button type="button" aria-label={I18n.get('Shutdown')} onClick={()=>runConfirmedCommand('~T3')} className='btn btn-outline btn-square base-content sm:btn-sm' disabled={!present || commandPending}><AiOutlineClose aria-hidden="true"/></button>
          </div>
        </div>
      </div>
    </section>

    {commandFeedback && (
      <div
        role={commandFeedback.type === "error" ? "alert" : "status"}
        aria-live="polite"
        className={`alert mt-4 break-words ${commandFeedback.type === "error" ? "alert-error" : "alert-success"}`}
      >
        {commandFeedback.text}
      </div>
    )}

    <section aria-label={I18n.get('Monitoring settings')} className="card mt-4 w-full space-y-3 rounded-2xl border border-base-300/70 bg-base-100 p-4 shadow-lg sm:space-y-4 sm:p-5">
        <label className="grid min-h-12 cursor-pointer grid-cols-[minmax(0,1fr)_auto] items-center gap-3 sm:min-h-0 sm:grid-cols-[minmax(0,1fr)_minmax(8rem,20rem)_auto]">
          <span className="label-text min-w-0">{I18n.get('TCP endpoint monitoring')}</span>
          <input aria-label={I18n.get('TCP endpoint monitoring')} title={I18n.get('Host, host:port or URL. A plain host uses port 80.')} placeholder="host, host:port, https://host…" type="text" name="network-monitoring" autoComplete="off" value={settings.Net} onChange={(e)=>{setSettings({Net: e.target.value})}} className="input input-bordered input-accent col-span-2 row-start-2 w-full min-w-0 sm:input-sm sm:row-auto sm:col-span-1 sm:col-start-2" disabled={!present}/>
          <input aria-label={`${I18n.get('TCP endpoint monitoring')}: ${I18n.get('Host, host:port or URL. A plain host uses port 80.')}`} checked={settings.NetEn} onChange={()=>{setSettings({NetEn: !settings.NetEn})}} type="checkbox" className="toggle toggle-accent col-start-2 row-start-1 sm:row-auto sm:col-start-3" disabled={!present}/>
        </label>

        <div className="grid min-h-12 grid-cols-[minmax(0,1fr)_auto] items-center gap-3 sm:min-h-0 sm:grid-cols-[minmax(0,1fr)_minmax(8rem,20rem)_auto]">
          <span className="label-text min-w-0">{I18n.get('Process monitoring')}</span>
          <div className="col-span-2 row-start-2 sm:row-auto sm:col-span-1 sm:col-start-2">
            <ProcDialog proc={settings.Proc} onChange={(name)=>{setSettings({Proc: name})}} disabled={!present}/>
          </div>
          <label className="col-start-2 row-start-1 flex min-h-11 cursor-pointer items-center justify-end sm:row-auto sm:col-start-3">
            <span className="sr-only">{I18n.get('Process monitoring')}</span>
            <input aria-label={I18n.get('Process monitoring')} checked={settings.ProcEn} onChange={()=>{setSettings({ProcEn: !settings.ProcEn})}} type="checkbox" className="toggle toggle-accent" disabled={!present}/>
          </label>
        </div>

        <label className="grid min-h-12 cursor-pointer grid-cols-[minmax(0,1fr)_auto] items-center gap-3">
          <span className="label-text min-w-0">{I18n.get('Led')}</span>
          <input aria-label={I18n.get('Led')} checked={settings.Diode} onChange={()=>{void setDiode()}} type="checkbox" className="toggle toggle-accent" disabled={!present || commandPending}/>
        </label>

        <label className="grid min-h-12 cursor-pointer grid-cols-[minmax(0,1fr)_auto] items-center gap-3">
          <span className="label-text min-w-0">{I18n.get('Pause')}</span>
          <input aria-label={I18n.get('Pause')} checked={settings.Pause} onChange={()=>{void setPause()}} type="checkbox" className="toggle toggle-accent" disabled={!present || commandPending}/>
        </label>
    </section>

  </div>
  )
}
