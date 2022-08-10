import './index.css';
import React, { useState, useEffect } from "react";
import TabMain from './TabMain';
import TabSettings from './TabSettings';
import TabConnect from './TabConnect';
import axios from 'axios';
import I18n from './I18n';
import Login from './Login';

axios.defaults.baseURL =  `http://${document.location.hostname}:8000`

export default function App () {
  const [ settings, setSettings ] = useState({ ConEn: false, Net: "", NetEn: false, Proc: "", ProcEn: false,
                                               Diode: true, Pause: false, ConUID: "", ConDev: "", ConAlias: "",
                                               ConAlert: false, ConAlertVal: 0, ConAlertSens: 0, ConAlertTimeout: 0, Empty: true });

  const [showSettings, setShowSettings] = useState(false);
  const [present, setPresent] = useState(false);

  const updateSettings = (val)=> {
    setSettings(settings => ({
      ...settings,
      ...val
    }));
  }

  useEffect(() => {
    if (settings.Empty === true) {axios.get('/settings').then((res)=>{
      setSettings(res.data);
    })} else { 
      axios.post('/settings', settings)
    }
  }, [settings]);

  return ( <div className="bg-repeat bg-base-200 p-6 w-full h-screen" style={{backgroundImage: "radial-gradient(hsla(var(--bc) /.2) .5px, hsla(var(--b2) /1) .5px)", backgroundSize: "5px 5px"}}>
              <div  className="h-full overflow-y-auto">
                <div className="divider mt-0">{I18n.get('Main')}</div>
                <TabMain settings={settings} setSettings={updateSettings} setPresentExt={setPresent}/>
                <div className="divider mt-8">
                        {I18n.get('Settings')}<input checked={present && showSettings} onChange={()=>{setShowSettings(!showSettings)}} type="checkbox" className="toggle toggle-sm" disabled={!present}/> 
                        Connect<input checked={present && settings.ConEn} onChange={()=>{updateSettings({ConEn: !settings.ConEn})}} type="checkbox" className="toggle toggle-sm" disabled={!present}/> 
                    </div>
                  { showSettings && <TabSettings/> }
                  { settings.ConEn && 
                    <>
                      { settings.ConUID ? <TabConnect settings={settings} setSettings={updateSettings}/> : <Login settings={settings} setSettings={updateSettings}/> }
                    </>
                  }
              </div>
            </div>

  );
}
