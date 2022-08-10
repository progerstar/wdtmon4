import './index.css';
import React, { useState, useEffect } from "react";
import { AiOutlineClose } from "react-icons/ai";
import axios from 'axios';
import I18n from './I18n';

const sensVariant = [I18n.get("Load"), I18n.get("Temperature")];

export default function TabConnect(props) {
    const { settings, setSettings } = props;
    const [balance, setBalance] = useState();

    useEffect(()=> {
        if ((settings.ConUID === "")) { return }
        axios.get('/con/user').then(function (response) {
                setBalance(response.data.balance);
        })
    }, [settings.ConUID]);

    const clear =()=>{
        setSettings({ConUID: ""})
    }

  return ( <div>
    <div>
            <div className="card p-2 mb-4 pr-4 w-full bg-base-100 rounded-xl shadow-xl flex flex-row items-center space-x-8">                
                <span className="ml-2 blur-sm hover:blur-none">ID: {settings.ConUID}</span>
                <div className="grow"/>
                <span>{I18n.get('Balance')}: {balance}</span>
                <a onClick={()=>{}} className="link link-accent">{I18n.get('Account')}</a>
                <button onClick={clear} className="btn btn-sm btn-square btn-outline btn-primary">
                    <AiOutlineClose/>
                </button>
            </div>
        
            <div className="card p-4 w-full bg-base-100 rounded-xl shadow-xl flex flex-col">
                <div className="form-control mt-4">
                    <label className="label cursor-pointer">
                        <span className="label-text">{I18n.get('Device')}</span>
                        <input type="text" value={settings.ConAlias} onChange={(e)=>{ setSettings({ConAlias: e.target.value})} } className="input input-bordered input-accent input-xs w-full max-w-xs" />
                    </label>
                </div>

                <div className="form-control">
                    <label className="label cursor-pointer">
                        <span className="label-text">{ I18n.get('Alert') }</span>
                        <input checked={settings.ConAlert} onChange={()=>{
                            if ((settings.ConAlert === false) && (balance <=0)) {
                                alert(I18n.get('For this function your balance must be greater than 0'))
                                return;
                            }
                            setSettings({ConAlert: !settings.ConAlert})}
                            } type="checkbox" className="toggle toggle-accent" />
                    </label>
                </div>

                <div className="form-control">
                    <label className="label cursor-pointer">
                        <span className="label-text">{I18n.get('Source')}</span>
                        <select value={sensVariant[settings.ConAlertSens]} disabled={!settings.ConAlert} onChange={(e)=>{
                            setSettings({ConAlertSens: sensVariant.indexOf(e.target.value)})
                        }} className="select select-bordered select-accent select-xs w-full max-w-xs">
                            <option>{sensVariant[0]}</option>
                            <option>{sensVariant[1]}</option>
                        </select>

                    </label>
                </div>

                <div className="form-control">
                    <label className="label cursor-pointer">
                        <span className="label-text">{I18n.get('Value')}</span>
                        <input type="text" value={settings.ConAlertVal} disabled={!settings.ConAlert} onChange={(e)=>{
                            setSettings({ConAlertVal: parseInt(e.target.value)})
                        }} placeholder="Value" className="input input-bordered input-accent input-xs w-full max-w-xs" />
                    </label>
                </div>

                <div className="form-control">
                    <label className="label cursor-pointer">
                        <span className="label-text">{I18n.get('Period')}</span>
                        <input type="text" value={settings.ConAlertTimeout} disabled={!settings.ConAlertVal} onChange={(e)=>{
                             setSettings({ConAlertTimeout: parseInt(e.target.value)})
                            }} placeholder="Minutes" className="input input-bordered input-accent input-xs w-full max-w-xs" />
                    </label>
                </div>

            </div>
    </div>
  </div>

  )
}