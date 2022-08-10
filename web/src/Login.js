import './index.css';
import React, { useState, useRef } from "react";
import axios from 'axios';
import I18n from './I18n';

export default function Login(props) {
  const { settings, setSettings } = props;
  const [balance, setBalance] = useState();
  const inputRef = useRef();

  const login =()=>{
    axios.get('/con/user', {
      headers: {
        'id': inputRef.current.value
      }}).then(function (res) {
          setSettings({ConUID: inputRef.current.value})
      })
  }

  const create =()=>{
    axios.get('/con/create').then((res)=> {
          setSettings({ConUID: res.data.id})
        })
        .catch(function (error) {
          // handle error
          console.log(error);
        })
        .then(function () {
          // always executed
        });
  }

  return (
      <div className="hero rounded-xl shadow-xl">
        <div className="hero-content flex-row">

        <div className="card flex-shrink-0 w-full max-w-sm shadow-2xl bg-base-100 mr-20">
            <div className="card-body">
              <div className="form-control">
                <label className="label">
                  <span className="label-text">UID</span>
                </label>
                <input type="text" ref={inputRef} className="input input-bordered" />
              </div>
              <div className="form-control mt-4">
                <label className="label">
                  <span className="label-text">{I18n.get("Don't have an account?")}</span>
                  <a onClick={create} href="#" className="label-text-alt link link-hover">{I18n.get('Register')}</a>
                </label>
              </div>
              <div onClick={login} className="form-control mt-6">
                <button className="btn btn-primary">{I18n.get('Login')}</button>
              </div>
            </div>
          </div>

          <div className="text-center lg:text-left">
            <h1 className="text-5xl font-bold">{I18n.get('Login to Connect!')}</h1>
            <p className="py-6">{I18n.get('Simple and easy to use cloud system')}</p>
          </div>

        </div>
      </div>
  )
}