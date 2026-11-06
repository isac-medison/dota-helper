import { useState, useRef } from 'react'
import {GreetService} from "../bindings/changeme";

import { Window } from '@wailsio/runtime';

// Show the actual Wails version this project was generated against.

function App() {
  const [name, setName] = useState<string>('');
  const titleNameRef = useRef<HTMLSpanElement | null>(null);
  const toastRef = useRef<HTMLDivElement | null>(null);
  const resultRef = useRef<HTMLSpanElement | null>(null);
  const toastTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  // Crossfade the framework word in the heading ("Wails + React") to the name
  // the user entered ("Wails + <name>"): the old word fades out while the new one
  // fades in over the same spot.
  const swapTitleName = (name: string) => {
    const titleNameElement = titleNameRef.current;
    if (!titleNameElement) {
      return;
    }
    const current = titleNameElement.querySelector('.title-name-text:not(.is-outgoing)');
    if (!current || current.textContent === name) {
      return;
    }
    const incoming = document.createElement('span');
    incoming.className = 'title-name-text is-entering';
    incoming.textContent = name;
    current.classList.add('is-outgoing');
    titleNameElement.appendChild(incoming);
    // Force a reflow so the transitions run from the starting state.
    void incoming.offsetWidth;
    incoming.classList.remove('is-entering');
    current.classList.add('is-leaving');
    current.addEventListener('transitionend', () => current.remove(), {once: true});
  };

  // Pop the toast with the message Go returned, then auto-dismiss it.
  const showToast = (message: string) => {
    if (resultRef.current) {
      resultRef.current.innerText = message;
    }
    if (toastRef.current) {
      toastRef.current.classList.add('is-visible');
    }
    clearTimeout(toastTimer.current);
    toastTimer.current = setTimeout(() => {
      if (toastRef.current) {
        toastRef.current.classList.remove('is-visible');
      }
    }, 4000);
  };

  const doGreet = () => {
    let n = name || 'anonymous';
    swapTitleName(n);
    GreetService.Greet(n).then(showToast).catch(console.error);
  };


  return (
    <div style={{ backgroundColor: '#000000', height: '100vh', color: 'white' }}>
    <div className="titlebar">
              <div className="titlebar-title">Dota Helper</div>

              {/* Window Controls Container */}
              <div className="titlebar-controls">
                  <button 
                    className="titlebar-button" 
                    onClick={() => Window.Minimise()}
                    title="Minimize"
                  >
                      &#x2012;
                  </button>
                  
                  <button 
                    className="titlebar-button" 
                    onClick={() => Window.ToggleMaximise()}
                    title="Maximize"
                  >
                      &#x25A1;
                  </button>
                  
                  <button 
                    className="titlebar-button titlebar-close" 
                    onClick={() => Window.Close()}
                    title="Close"
                  >
                      &#x2715;
                  </button>
              </div>
          </div>

      {/* Your main app content goes here */}
      <div style={{ padding: '20px' }}>
          <main className="container">
        <h1 className="title"><span className="title-accent">Wails +</span> <span className="title-name" ref={titleNameRef}><span className="title-name-text">React</span></span></h1>

        <div className="greet">
          <div className="input-box">
            <input aria-label="input" className="input" value={name} onChange={(e) => setName(e.target.value)} type="text" placeholder="Your name" autoComplete="off"/>
            <button aria-label="greet-btn" className="btn" onClick={doGreet}>Greet
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><line x1="5" y1="12" x2="19" y2="12"/><polyline points="12 5 19 12 12 19"/></svg>
            </button>
          </div>
        </div>
      </main>

      </div>

      

    </div>
  )
}

export default App
