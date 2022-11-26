import './index.css';
import React from "react";
import { useState } from 'react';
import I18n from './I18n';
import axios from 'axios';
import useInterval from './useInterval';
import ProcDialog from './ProcDialog';
import { AiOutlineReload, AiOutlinePoweroff, AiOutlineClose } from "react-icons/ai";

const NODEV = "------------";
const NOTEMP = "--.--";

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

  const [uptime, setUptime] = useState(null);
  const [temp, setTemp] = useState(NOTEMP);

  useInterval(() => {
    axios.get('/uptime').then((res)=>{
      setUptime(formatDate(res.data))
    });
  }, 1000);

  useInterval(() => {
    axios.get('/cmd/~U').then((res)=>{
      const data = res.data;
      if (data.startsWith('~A')) {
        axios.get('/cmd/~G').then((res)=>{
          setTemp(res.data)
        }).catch(()=>{setTemp(NOTEMP)})
      }
    })
  }, 3500);

  useInterval(() => {
    axios.get('/cmd/~G').then((res)=>{
      setTemp(res.data)
    }).catch(()=>{setTemp(NOTEMP)})
  }, 5500);

  useInterval(() => {
      axios.get('/cmd/~I').then((res)=>{
        const data = res.data;
        if (data.startsWith('~I')) {
          setInfo(data.slice(2))
          setPresent(true);
          setPresentExt(true);
        }
    }).catch(()=>{
      setInfo(NODEV);
      setPresent(false);
      setPresentExt(false);
    })
  }, 8500);

  const setDiode =()=> {
    axios.get('/cmd/~L'+(!settings.Diode? "1":"0")).then((res)=>{
      if (res.data.startsWith('~L')) {
        setSettings({Diode: res.data[2] === "1"})
      }
      
    }).catch(()=>{})
  }

  const setPause =()=> {
    axios.get('/cmd/~P'+(!settings.Pause? "1":"0")).then((res)=>{
      //console.log(res.data);
      if (res.data.startsWith('~P')) {
        setSettings({Pause: res.data[2] === "1"})
      }
    }).catch(()=>{})
  }

  return (<div>
    <div className="card w-full bg-base-100 h-12 rounded-xl shadow-xl flex flex-row items-center">
      <p className="ml-8">{info}</p>        

      <div className="flex items-center justify-center ml-8">
          <span className="flex absolute h-4 w-4">
            { present && <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-accent opacity-75"></span> }
            <span className="relative inline-flex rounded-full h-4 w-4 bg-accent"></span>
          </span>
      </div>

      <div className="grow"/>
      <div className={"space-x-4 flex flex-row mr-4"}>
        <p className="">{uptime}</p> 
        <p className="">{temp}°C</p>
        <a href="/monitor" target="_blank" rel="noreferrer" className="link link-accent link-hover">{I18n.get("Monitor")}</a>
        <div className="tooltip tooltip-left" data-tip='Reset'>
          <button onClick={()=>{ axios.get('/cmd/~T1') }} className='btn btn-outline btn-square base-content btn-xs' disabled={!present}><AiOutlineReload/></button>
        </div>
        <div className="tooltip tooltip-left" data-tip='Power'>
          <button onClick={()=>{ axios.get('/cmd/~T2') }} className='btn btn-outline btn-square base-content btn-xs' disabled={!present}><AiOutlinePoweroff/></button>
        </div>
        <div className="tooltip tooltip-left" data-tip='Off'>
          <button onClick={()=>{ axios.get('/cmd/~T3') }} className='btn btn-outline btn-square base-content btn-xs' disabled={!present}><AiOutlineClose/></button>
        </div>
      </div>
    </div>


    <div className={"card w-full bg-base-100 shadow-xl mt-4 p-4 flex-col space-y-4"}>
        <div className="flex justify-between">
          <span className="label-text">{I18n.get('Network monitoring')}</span>
          <input type="text" value={settings.Net} onChange={(e)=>{setSettings({Net: e.target.value})}} className="input input-bordered input-accent input-xs w-full max-w-xs" disabled={!present}/>
          <input checked={settings.NetEn} onChange={()=>{setSettings({NetEn: !settings.NetEn})}} type="checkbox" className="toggle toggle-accent" disabled={!present}/>
        </div>

        <div className="flex justify-between">
          <span className="label-text">{I18n.get('Process monitoring')}</span>
          <ProcDialog proc={settings.Proc} onChange={(name)=>{setSettings({Proc: name})}} disabled={!present}/>
          <input checked={settings.ProcEn} onChange={()=>{setSettings({ProcEn: !settings.ProcEn})}} type="checkbox" className="toggle toggle-accent" disabled={!present}/>
        </div>

        <div className="flex justify-between">
          <span className="label-text">{I18n.get('Led')}</span>
          <input checked={settings.Diode} onChange={()=>{setDiode();setSettings({Diode: !settings.Diode})}} type="checkbox" className="toggle toggle-accent" disabled={!present}/>
        </div>

        <div className="flex justify-between">
          <span className="label-text">{I18n.get('Pause')}</span>
          <input checked={settings.Pause} onChange={()=>{setPause(); setSettings({Pause: !settings.Pause})}} type="checkbox" className="toggle toggle-accent" disabled={!present}/>
        </div>
    </div>

  </div>
  )
}



