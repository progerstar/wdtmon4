import { useCallback, useEffect, useRef, useState } from "react";
import I18n from './I18n';
import axios from 'axios';
import toast, { Toaster } from 'react-hot-toast';

const DEVICE_SETTINGS_TIMEOUT = 3000;
const HEX_DIGIT = /^[0-9A-Fa-f]$/;
const DECIMAL_DIGIT = /^[0-9]$/;
const HEX_BYTE = /^[0-9A-Fa-f]{2}$/;

const renderTxSelect =(cur, mul, txt, cb, label)=> {
    if (cur === null) return null;
    const val = cur*mul+' '+txt;
    const rangeList = [...Array(16).keys()].map((el)=>el*mul+' '+txt);
    const rows = rangeList.map((el,i)=>{ return <option key={i}>{el}</option> });
    return <select aria-label={label} value={val} onChange={(e)=>{cb(rangeList.indexOf(e.target.value))}} className="select select-bordered w-full max-w-xs sm:select-sm">
                {rows}
            </select>
};

const renderLimitSelect =(cur, cb, label)=> {
    if (cur === null) return null;
    const rows = [...Array(16).keys()].map(el=>{ return <option key={el}>{el}</option> });
    return <select aria-label={label} value={cur} onChange={(e)=>{cb(parseInt(e.target.value, 10))}} className="select select-bordered w-full max-w-xs sm:select-sm">
                {rows}
            </select>
};

const ChSelectValues = [I18n.get("Off"), I18n.get("Reset"), I18n.get("Power"), I18n.get("Out opened"), I18n.get("Out closed")];
const renderChSelect =(cur, cb, label)=> {
    if (cur === null) return null;
    const rows = ChSelectValues.map((el, i)=>{ return <option key={i}>{el}</option> });
    return <select aria-label={label} value={ChSelectValues[cur]} onChange={(e)=>{cb(ChSelectValues.indexOf(e.target.value))}} className="select select-bordered w-full max-w-xs sm:select-sm">
        {rows}
    </select>
};

const InSelectValues = [I18n.get("Off"), I18n.get("Input"),I18n.get("Reserved"), I18n.get("Temp.sensor")];
const renderInSelect =(cur, cb, label)=> {
    if (cur === null) return null;
    const rows = InSelectValues.map((el, i)=>{ return <option key={i}>{el}</option> });
    return <select aria-label={label} value={InSelectValues[cur]} onChange={(e)=>{cb(InSelectValues.indexOf(e.target.value))}} className="select select-bordered w-full max-w-xs sm:select-sm">
        {rows}
    </select>
};


function showToast(text) {
    toast.custom((t) => ( <div role="status" aria-live="polite" className={`${t.visible ? 'animate-enter' : 'animate-leave' }
        card bg-base-300 w-56 min-h-10 px-4 py-2 shadow-lg rounded-lg pointer-events-auto flex items-center justify-center`}>
    {text}
    </div>))
}

function parseInteger(text, base, max, pattern) {
    if (typeof text !== "string" || !pattern.test(text)) return null;
    const value = Number.parseInt(text, base);
    return Number.isInteger(value) && value >= 0 && value <= max ? value : null;
}

export function decodeSettingsResponse(response) {
    const command = String(response ?? "");
    if (!command.startsWith('~F') || (command.length !== 4 && command.length !== 13)) {
        return null;
    }

    const decoded = {
        t1: parseInteger(command[2], 16, 15, HEX_DIGIT),
        t2: parseInteger(command[3], 16, 15, HEX_DIGIT),
    };
    if (decoded.t1 === null || decoded.t2 === null) {
        return null;
    }
    if (command.length === 4) {
        return decoded;
    }

    Object.assign(decoded, {
        t3: parseInteger(command[4], 16, 15, HEX_DIGIT),
        t4: parseInteger(command[5], 16, 15, HEX_DIGIT),
        t5: parseInteger(command[6], 16, 15, HEX_DIGIT),
        ch1: parseInteger(command[7], 10, ChSelectValues.length - 1, DECIMAL_DIGIT),
        ch2: parseInteger(command[8], 10, ChSelectValues.length - 1, DECIMAL_DIGIT),
        limit: parseInteger(command[9], 16, 15, HEX_DIGIT),
        inp: parseInteger(command[10], 10, InSelectValues.length - 1, DECIMAL_DIGIT),
        temp: parseInteger(command.slice(11, 13), 16, 255, HEX_BYTE),
    });

    return Object.values(decoded).some((value) => value === null) ? null : decoded;
}

export function buildSettingsCommand(values) {
    const {
        t1, t2, t3, t4, t5, ch1, ch2, limit, inp, temp,
    } = values;
    const nibbles = [t1, t2, t3, t4, t5, limit];
    if (nibbles.some((value) => !Number.isInteger(value) || value < 0 || value > 15)) {
        return null;
    }
    if (![ch1, ch2].every((value) => Number.isInteger(value) && value >= 0 && value < ChSelectValues.length)) {
        return null;
    }
    if (!Number.isInteger(inp) || inp < 0 || inp >= InSelectValues.length) {
        return null;
    }
    if (!Number.isInteger(temp) || temp < 0 || temp > 255) {
        return null;
    }

    const hex = (value) => value.toString(16).toUpperCase();
    return `~W${hex(t1)}${hex(t2)}${hex(t3)}${hex(t4)}${hex(t5)}${ch1}${ch2}${hex(limit)}${inp}${hex(temp).padStart(2, '0')}`;
}

export default function TabSettings() {
    const [t1, setT1] = useState(null);
    const [t2, setT2] = useState(null);
    const [t3, setT3] = useState(null);
    const [t4, setT4] = useState(null);
    const [t5, setT5] = useState(null);
    const [ch1, setCh1] = useState(null);
    const [ch2, setCh2] = useState(null);
    const [limit, setLimit] = useState(null);
    const [inp, setInp] = useState(null);
    const [temp, setTemp] = useState(0);
    const [operation, setOperation] = useState("");
    const [operationError, setOperationError] = useState("");
    const requestControllerRef = useRef(null);
    const mountedRef = useRef(true);
    const busy = operation !== "";

    useEffect(() => {
        mountedRef.current = true;
        return () => {
            mountedRef.current = false;
            requestControllerRef.current?.abort();
        };
    }, []);

    const beginRequest = useCallback(() => {
        requestControllerRef.current?.abort();
        const controller = new AbortController();
        requestControllerRef.current = controller;
        return controller;
    }, []);

    const applySettings = useCallback((decoded) => {
        setT1(decoded.t1);
        setT2(decoded.t2);
        if (Object.hasOwn(decoded, "t3")) {
            setT3(decoded.t3);
            setT4(decoded.t4);
            setT5(decoded.t5);
            setCh1(decoded.ch1);
            setCh2(decoded.ch2);
            setLimit(decoded.limit);
            setInp(decoded.inp);
            setTemp(decoded.temp);
        }
    }, []);

    const read = useCallback(async (notify = true) => {
        setOperation("read");
        const controller = beginRequest();
        try {
            const response = await axios.post('/cmd/~F', undefined, {
                timeout: DEVICE_SETTINGS_TIMEOUT,
                signal: controller.signal,
            });
            if (controller.signal.aborted || !mountedRef.current) return;
            const decoded = decodeSettingsResponse(response.data);
            if (decoded === null) {
                const message = I18n.get('Unexpected device response');
                setOperationError(message);
                if (notify) showToast(message);
                return;
            }
            applySettings(decoded);
            setOperationError("");
            if (notify) showToast(I18n.get('Settings read'));
        } catch {
            if (controller.signal.aborted || !mountedRef.current) return;
            const message = I18n.get('Settings read failed');
            setOperationError(message);
            if (notify) showToast(message);
        } finally {
            if (requestControllerRef.current === controller) requestControllerRef.current = null;
            if (mountedRef.current && !controller.signal.aborted) setOperation("");
        }
    }, [applySettings, beginRequest]);

    const write = async () => {
        const command = buildSettingsCommand({
            t1, t2, t3, t4, t5, ch1, ch2, limit, inp, temp: Number(temp),
        });
        if (command === null) {
            const message = I18n.get('Wrong parameters');
            setOperationError(message);
            showToast(message);
            return;
        }

        setOperation("write");
        const controller = beginRequest();
        try {
            const response = await axios.post('/cmd/' + command, undefined, {
                timeout: DEVICE_SETTINGS_TIMEOUT,
                signal: controller.signal,
            });
            if (controller.signal.aborted || !mountedRef.current) return;
            const decoded = decodeSettingsResponse(response.data);
            if (decoded === null) {
                const message = response.data === 'Error' ? I18n.get('Error') : I18n.get('Unexpected device response');
                setOperationError(message);
                showToast(message);
                return;
            }
            applySettings(decoded);
            setOperationError("");
            showToast(I18n.get('Settings updated'));
        } catch {
            if (controller.signal.aborted || !mountedRef.current) return;
            const message = I18n.get('Error');
            setOperationError(message);
            showToast(message);
        } finally {
            if (requestControllerRef.current === controller) requestControllerRef.current = null;
            if (mountedRef.current && !controller.signal.aborted) setOperation("");
        }
    };

    useEffect(() => {
        void read(false);
    }, [read]);

  return (
        <div>
            <Toaster position="top-center" reverseOrder={false} containerStyle={{left: '1rem', right: '1rem'}}/>
            {operationError && <div role="alert" aria-live="polite" className="alert alert-error mb-4 break-words">{operationError}</div>}
            <div className="card mb-4 flex w-full flex-col rounded-2xl border border-base-300/70 bg-base-100 p-4 shadow-lg sm:p-5">
                <div className="divide-y divide-base-200">
                    <div className="grid gap-2 py-3 sm:grid-cols-[minmax(0,1fr)_minmax(10rem,16rem)] sm:items-center">
                        <div className="min-w-0">{I18n.get('PC will be restarted if there has been no signal from the app for')}</div>
                        <div>{renderTxSelect(t1, 1, I18n.get('min.'), setT1, I18n.get('PC will be restarted if there has been no signal from the app for'))}</div>
                    </div>

                    <div className="grid gap-2 py-3 sm:grid-cols-[minmax(0,1fr)_minmax(10rem,16rem)] sm:items-center">
                        <div className="min-w-0">{I18n.get('When restarting the PC, hold the "Reset" button for')}</div>
                        <div>{renderTxSelect(t2, 100, I18n.get('msec.'), setT2, I18n.get('When restarting the PC, hold the "Reset" button for'))}</div>
                    </div>

                    <div className="grid gap-2 py-3 sm:grid-cols-[minmax(0,1fr)_minmax(10rem,16rem)] sm:items-center">
                        <div className="min-w-0">{I18n.get('When hard-restarting the PC, hold the "Power" button for')}</div>
                        <div>{renderTxSelect(t3, 1, I18n.get('sec.'), setT3, I18n.get('When hard-restarting the PC, hold the "Power" button for'))}</div>
                    </div>

                    <div className="grid gap-2 py-3 sm:grid-cols-[minmax(0,1fr)_minmax(10rem,16rem)] sm:items-center">
                        <div className="min-w-0">{I18n.get('When hard-restarting the PC, after powering off, wait for')}</div>
                        <div>{renderTxSelect(t4, 1, I18n.get('sec.'), setT4, I18n.get('When hard-restarting the PC, after powering off, wait for'))}</div>
                    </div>

                    <div className="grid gap-2 py-3 sm:grid-cols-[minmax(0,1fr)_minmax(10rem,16rem)] sm:items-center">
                        <div className="min-w-0">{I18n.get('When hard-restarting the PC, after powering off, hold the "Power" button for')}</div>
                        <div>{renderTxSelect(t5, 100, I18n.get('msec.'), setT5, I18n.get('When hard-restarting the PC, after powering off, hold the "Power" button for'))}</div>
                    </div>

                    <div className="grid gap-2 py-3 sm:grid-cols-[minmax(0,1fr)_minmax(10rem,16rem)] sm:items-center">
                        <div className="min-w-0">{I18n.get('Channel 1')}</div>
                        <div>{renderChSelect(ch1, setCh1, I18n.get('Channel 1'))}</div>
                    </div>

                    <div className="grid gap-2 py-3 sm:grid-cols-[minmax(0,1fr)_minmax(10rem,16rem)] sm:items-center">
                        <div className="min-w-0">{I18n.get('Channel 2')}</div>
                        <div>{renderChSelect(ch2, setCh2, I18n.get('Channel 2'))}</div>
                    </div>

                    <div className="grid gap-2 py-3 sm:grid-cols-[minmax(0,1fr)_minmax(10rem,16rem)] sm:items-center">
                        <div className="min-w-0">{I18n.get('Channel IN')}</div>
                        <div>{renderInSelect(inp, setInp, I18n.get('Channel IN'))}</div>
                    </div>

                    <div className="grid gap-2 py-3 sm:grid-cols-[minmax(0,1fr)_minmax(10rem,16rem)] sm:items-center">
                        <div className="min-w-0">{I18n.get('Reset Limit')}</div>
                        <div>{renderLimitSelect(limit, setLimit, I18n.get('Reset Limit'))}</div>
                    </div>

                    <div className="grid gap-2 py-3 sm:grid-cols-[minmax(0,1fr)_minmax(10rem,16rem)] sm:items-center">
                        <div className="min-w-0">{I18n.get('Temperature Threshold')}</div>
                        <div><input aria-label={I18n.get('Temperature Threshold')} name="temperature-threshold" type="number" inputMode="numeric" min={0} max={255} step={1} value={temp} onChange={(e)=>{setTemp(Number(e.target.value))}} className="input input-bordered w-full max-w-xs sm:input-sm"/></div>
                    </div>
                </div>
                <div className="mt-4 grid grid-cols-2 gap-2 sm:flex sm:justify-end">
                    <button type="button" onClick={()=>{void read(true)}} className="btn btn-outline sm:btn-sm" disabled={busy}>{operation === "read" ? I18n.get('Reading…') : I18n.get('Read')}</button>
                    <button type="button" onClick={()=>{void write()}} className="btn btn-outline sm:btn-sm" disabled={busy}>{operation === "write" ? I18n.get('Writing…') : I18n.get('Write')}</button>
                </div>

            </div>
    </div>


  )
}
