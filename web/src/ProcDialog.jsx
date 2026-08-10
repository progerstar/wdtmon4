import { useCallback, useEffect, useRef, useState } from "react";
import I18n from './I18n';
import axios from 'axios';

const PROCESS_REQUEST_TIMEOUT = 5000;

export default function ProcDialog(props) {
    const {proc, onChange, disabled} = props;
    const [processes, setProcesses] = useState([]);
    const [error, setError] = useState("");
    const [loading, setLoading] = useState(false);
    const [open, setOpen] = useState(false);
    const triggerRef = useRef(null);
    const dialogRef = useRef(null);
    const closeButtonRef = useRef(null);
    const requestRef = useRef(null);
    const requestIdRef = useRef(0);

    const closeDialog = useCallback(() => {
        requestIdRef.current += 1;
        requestRef.current?.abort();
        requestRef.current = null;
        setOpen(false);
        if (!disabled) triggerRef.current?.focus();
    }, [disabled]);

    const loadProcesses = useCallback(async () => {
        if (disabled) return;

        requestRef.current?.abort();
        const controller = new AbortController();
        const requestId = requestIdRef.current + 1;
        requestIdRef.current = requestId;
        requestRef.current = controller;
        setLoading(true);
        setError("");
        try {
            const res = await axios.get('/proc', {
                timeout: PROCESS_REQUEST_TIMEOUT,
                signal: controller.signal,
            });
            if (controller.signal.aborted || requestIdRef.current !== requestId) return;
            const list = Array.isArray(res.data)
                ? res.data.filter((item) => item && typeof item.name === "string" && item.name !== "")
                : [];
            list.sort((a, b) => String(a.name).localeCompare(String(b.name)));
            setProcesses(list);
        } catch {
            if (controller.signal.aborted || requestIdRef.current !== requestId) return;
            setProcesses([]);
            setError(I18n.get('Failed to load processes'));
        } finally {
            if (requestRef.current === controller) requestRef.current = null;
            if (!controller.signal.aborted && requestIdRef.current === requestId) setLoading(false);
        }
    }, [disabled]);

    const openDialog = () => {
        if (disabled) return;
        setOpen(true);
        void loadProcesses();
    };

    useEffect(() => {
        if (!open) return undefined;
        closeButtonRef.current?.focus();
        const closeOnEscape = (event) => {
            if (event.key === 'Escape') {
                event.preventDefault();
                closeDialog();
                return;
            }
            if (event.key !== 'Tab') return;

            const focusable = Array.from(dialogRef.current?.querySelectorAll(
                'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
            ) || []);
            if (focusable.length === 0) {
                event.preventDefault();
                dialogRef.current?.focus();
                return;
            }

            const first = focusable[0];
            const last = focusable[focusable.length - 1];
            const active = document.activeElement;
            if (event.shiftKey && (active === first || !dialogRef.current?.contains(active))) {
                event.preventDefault();
                last.focus();
            } else if (!event.shiftKey && (active === last || !dialogRef.current?.contains(active))) {
                event.preventDefault();
                first.focus();
            }
        };
        document.addEventListener('keydown', closeOnEscape);
        return () => document.removeEventListener('keydown', closeOnEscape);
    }, [closeDialog, open]);

    useEffect(() => {
        if (disabled && open) closeDialog();
    }, [closeDialog, disabled, open]);

    useEffect(() => () => requestRef.current?.abort(), []);

    return (
        <>
            <button ref={triggerRef} type="button" className={`input input-bordered input-accent w-full min-w-0 max-w-xs justify-start overflow-hidden text-left sm:input-sm ${proc ? '' : 'text-base-content/55'} disabled:cursor-not-allowed`} onClick={openDialog} disabled={disabled} aria-label={I18n.get('Open process list')} title={proc || I18n.get('Select process')}>
                <span className="min-w-0 truncate">{proc || I18n.get('Select process')}</span>
            </button>

            {open && <div className="modal modal-open p-3">
                <div ref={dialogRef} tabIndex={-1} className="modal-box relative flex max-h-[85dvh] w-full max-w-2xl flex-col overscroll-contain p-4 sm:p-6" role="dialog" aria-modal="true" aria-labelledby="process-dialog-title">
                    <div className="flex items-center justify-between gap-4 pr-1">
                        <h2 id="process-dialog-title" className="text-lg font-bold">{I18n.get('Select process')}</h2>
                        <button ref={closeButtonRef} type="button" aria-label={I18n.get('Close')} onClick={closeDialog} className="btn btn-circle btn-ghost min-h-11 shrink-0 sm:btn-sm sm:min-h-0">×</button>
                    </div>

                    <div className="mt-3 min-h-20 overflow-y-auto overscroll-contain rounded-xl border border-base-300">
                        <table className="table w-full text-sm">
                            <tbody aria-live="polite">
                                {loading && <tr><td className="text-center">{I18n.get('Loading…')}</td></tr>}
                                {!loading && error && <tr><td className="text-center text-error">{error}</td></tr>}
                                {!loading && !error && processes.length === 0 && <tr><td className="text-center">{I18n.get('No processes found')}</td></tr>}
                                {!loading && !error && processes.map((item, index) => (
                                    <tr key={`${item.name}-${item.pid ?? index}`} className="hover">
                                        <td className="whitespace-normal p-0">
                                            <button type="button" className="min-h-11 w-full break-all whitespace-normal px-4 py-3 text-left hover:bg-base-200 focus-visible:bg-base-200" onClick={()=>{onChange(item.name); closeDialog();}}>
                                                {item.name}
                                            </button>
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                </div>
                <button type="button" aria-label={I18n.get('Close dialog backdrop')} className="modal-backdrop cursor-default" onClick={closeDialog}></button>
            </div>}
        </>
    );
}
