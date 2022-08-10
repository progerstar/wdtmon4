import './index.css';
import React from "react";
import { useState, useEffect } from 'react';
import I18n from './I18n';
import axios from 'axios';

export default function ProcDialog(props) {
    const {proc, onChange, disabled} = props;
    const [table, setTable] = useState(null);

    useEffect(() => {
        axios.get('/proc').then((res)=>{
            const processes = res.data;
            if ((processes) && (processes.length > 0)) {
                processes.sort((a, b) => a.Name.localeCompare(b.Name));
                setTable(processes.map((proc, i) => {
                    return  <tr key={i} className="hover">
                                <td className="flex justify-start"><div className="modal-action"><a href="#"><p onClick={()=>{onChange(proc.Name)}}>{proc.Name}</p></a></div></td>
                            </tr>
                }))
            }
        })
    },[]);
    
    return (
        <>
            <a href={disabled? "":"#modal"} className='w-full max-w-xs'>
                <input type="text" value={proc} className=" cursor-pointer input input-bordered input-accent input-xs w-full" readOnly disabled={disabled}/>
            </a>

            <div id="modal" className="modal">
                    <div className="modal-box">
                    <div className="modal-action">
                        <a href="#" className="btn btn-sm btn-circle absolute right-2 top-2">✕</a>
                    </div>

            
                    <h3 className="text-lg font-bold mb-2">{I18n.get('Select process')}</h3>
                    <table className="table table-compact text-xs w-full">
                        <tbody>
                            {table}
                        </tbody>
                    </table>
                
                </div>
            </div>
        </>
    )
}