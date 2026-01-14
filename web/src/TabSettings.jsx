import './index.css';
import React, { useState, useEffect }  from "react";
import I18n from './I18n';
import axios from 'axios';
import toast, { Toaster } from 'react-hot-toast';

const renderTxSelect =(cur, mul, txt, cb)=> {
    if (cur === null) return null;
    const val = cur*mul+' '+txt;
    const rangeList = [...Array(16).keys()].map((el)=>el*mul+' '+txt);
    const rows = rangeList.map((el,i)=>{ return <option key={i}>{el}</option> });
    return <select value={val} onChange={(e)=>{cb(rangeList.indexOf(e.target.value))}} className="select select-ghost w-full max-w-xs select-xs">
                {rows}
            </select>
};

const renderLimitSelect =(cur, cb)=> {
    if (cur === null) return null;
    const rows = [...Array(16).keys()].map(el=>{ return <option key={el}>{el}</option> });
    return <select value={cur} onChange={(e)=>{cb(parseInt(e.target.value, 10))}} className="select select-ghost w-full max-w-xs select-xs">
                {rows}
            </select>
};

const ChSelectValues = [I18n.get("Off"), "Reset", "Power", I18n.get("Out opened"), I18n.get("Out closed")];
const renderChSelect =(cur, cb)=> {
    if (cur === null) return null;
    const rows = ChSelectValues.map((el, i)=>{ return <option key={i}>{el}</option> });
    return <select value={ChSelectValues[cur]} onChange={(e)=>{cb(ChSelectValues.indexOf(e.target.value))}} className="select select-ghost w-full max-w-xs select-xs">
        {rows}
    </select>
};

const InSelectValues = [I18n.get("Off"), I18n.get("Input"),I18n.get("Reserved"), I18n.get("Temp.sensor")];
const renderInSelect =(cur, cb)=> {
    if (cur === null) return null;
    const rows = InSelectValues.map((el, i)=>{ return <option key={i}>{el}</option> });
    return <select value={InSelectValues[cur]} onChange={(e)=>{cb(InSelectValues.indexOf(e.target.value))}} className="select select-ghost w-full max-w-xs select-xs">
        {rows}
    </select>
};


function showToast(text) {
    toast.custom((t) => ( <div className={`${t.visible ? 'animate-enter' : 'animate-leave' } 
        card bg-base-300 w-56 h-10 p-6 w-full shadow-lg rounded-lg pointer-events-auto flex items-start justify-center`}>
    {text}
    </div>))
}

export default function TabSettings() {
    const [t1, setT1] = useState(null)
    const [t2, setT2] = useState(null)
    const [t3, setT3] = useState(null)
    const [t4, setT4] = useState(null)
    const [t5, setT5] = useState(null)
    const [ch1, setCh1] = useState(null)
    const [ch2, setCh2] = useState(null)
    const [limit, setLimit] = useState(null)
    const [inp, setInp] = useState(null)
    const [temp, setTemp] = useState(0)
    
    const parse =(cmd)=> {
        if (cmd.startsWith('~F')) {
            setT1(parseInt(cmd[2], 16));
            setT2(parseInt(cmd[3], 16));
            if (cmd.length === 13) {
                setT3(parseInt(cmd[4], 16));
                setT4(parseInt(cmd[5], 16));
                setT5(parseInt(cmd[6], 16));
                setCh1(parseInt(cmd[7], 10));
                setCh2(parseInt(cmd[8], 10));
                setLimit(parseInt(cmd[9], 16));
                setInp(parseInt(cmd[10], 10));
                setTemp(parseInt(cmd.slice(11, 13), 16));
            }
        }
    }

    const read =()=> {
        axios.get('/cmd/~F').then((res)=>{
            if ((res.data.length===4) || (res.data.length===13)) {
                parse(res.data);
                showToast(I18n.get('Settings read'));
            }
        })
    }

    const write =()=> {
        const missing = [t1, t2, t3, t4, t5, ch1, ch2, limit, inp].some((v)=>v === null || v === undefined);
        if (missing) {showToast(I18n.get('Wrong parameters')); return;}
        const tempVal = Number(temp);
        if (!Number.isFinite(tempVal) || tempVal < 0 || tempVal > 255) {
            showToast(I18n.get('Wrong parameters'));
            return;
        }
        const tempHex = tempVal.toString(16).toUpperCase().padStart(2,'0');
        const formatT = (tX) => Number(tX).toString(16).toUpperCase();
        const s = `${formatT(t1)}${formatT(t2)}${formatT(t3)}${formatT(t4)}${formatT(t5)}${ch1}${ch2}${formatT(limit)}${inp}${tempHex}`;
        axios.get('/cmd/~W'+s).then((res)=>{
            if (res.data === 'Error') {
                showToast(I18n.get('Error'));
            } else {
                parse(res.data);
                showToast(I18n.get('Settings updated'));
            }
        }).catch(()=>{showToast(I18n.get('Error'));})
    }
        
    useEffect(() => {
        read();
      }, []);
      
  return (
        <div>
            <Toaster position="top-right" reverseOrder={false}/>
            <div className="card p-4 mb-4 w-full bg-base-100 rounded-xl shadow-xl flex flex-col">
                <table className="table table-compact w-full">
                <tbody>
                    <tr>
                        <td>{I18n.get('PC will be restarted if there has been no signal from the app for')}</td>
                        <td>{renderTxSelect(t1, 1, I18n.get('min.'), setT1)}</td>
                    </tr>

                    <tr>
                        <td>{I18n.get('When restarting the PC, hold the "Reset" button for')}</td>
                        <td>{renderTxSelect(t2, 100, I18n.get('msec.'), setT2)}</td>
                    </tr>

                    <tr>
                        <td>{I18n.get('When hard-restarting the PC, hold the "Power" button for')}</td>
                        <td>{renderTxSelect(t3, 1, I18n.get('sec.'), setT3)}</td>
                    </tr>

                    <tr>
                        <td>{I18n.get('When hard-restarting the PC, after powering off, wait for')}</td>
                        <td>{renderTxSelect(t4, 1, I18n.get('sec.'), setT4)}</td>
                    </tr>

                    <tr>
                        <td>{I18n.get('When hard-restarting the PC, after powering off, hold the "Power" button for')}</td>
                        <td>{renderTxSelect(t5, 100, I18n.get('msec.'), setT5)}</td>
                    </tr>

                    <tr>
                        <td>{I18n.get('Channel 1')}</td>
                        <td>{renderChSelect(ch1, setCh1)}</td>
                    </tr>

                    <tr>
                        <td>{I18n.get('Channel 2')}</td>
                        <td>{renderChSelect(ch2, setCh2)}</td>
                    </tr>

                    <tr>
                        <td>{I18n.get('Channel IN')}</td>
                        <td>{renderInSelect(inp, setInp)}</td>
                    </tr>

                    <tr>
                        <td>{I18n.get('Reset Limit')}</td>
                        <td>{renderLimitSelect(limit, setLimit)}</td>
                    </tr>

                    <tr>
                        <td>{I18n.get('Temperature Threshold')}</td>
                        <td><input type="number" min={0} max={255} value={temp} onChange={(e)=>{setTemp(Number(e.target.value))}} className="input input-ghost w-full max-w-xs input-xs"/></td>
                    </tr>
                </tbody>
                </table>
                <div className="mt-4 mx-2 space-x-2 flex justify-end">
                    <button onClick={read} className="btn btn-outline btn-sm">{I18n.get('Read')}</button>
                    <button onClick={write} className="btn btn-outline btn-sm">{I18n.get('Write')}</button>
                </div>

            </div>
    </div>


  )
}
