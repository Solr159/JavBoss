import React, { useState } from 'react'
import { createRoot } from 'react-dom/client'
import JavGrid from '@/features/jav/components/JavGrid'
import JavSettings from '@/features/settings/components/JavSettings'
import { useStore } from '@/store'
import '@/index.css'

const cases = ['landscape', 'min', 'max', 'below', 'above', 'portrait', 'square', 'missing']
const items = cases.map((code, index) => ({ id: index + 1, code, title: code }))
let savedConfig = { jav_portrait_mode: false }
const originalFetch = window.fetch
window.fetch = async (input, init) => {
  if (input === '/config') {
    if (init?.method === 'PATCH') {
      savedConfig = { ...savedConfig, ...JSON.parse(init.body) }
    }
    return Response.json(savedConfig)
  }
  return originalFetch(input, init)
}
useStore.setState({ config: savedConfig })

function Fixture() {
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [changingSource, setChangingSource] = useState(false)
  window.openSettings = () => setSettingsOpen(true)
  window.changeCoverSource = () => setChangingSource((value) => !value)
  window.reloadConfig = async () => {
    useStore.setState({ config: {} })
    await useStore.getState().loadConfig()
  }
  window.readConfig = () => useStore.getState().config

  return (
    <main className="mx-auto max-w-[1200px] p-4">
      <h1 className="mb-4 text-xl font-semibold">JAV 封面测试</h1>
      <JavGrid
        items={changingSource ? [{ id: 1, code: 'portrait' }, ...items.slice(1)] : items}
        columns={4}
        titleMaxRows={0}
      />
      {settingsOpen ? (
        <JavSettings
          initialTab="jav"
          onClose={() => setSettingsOpen(false)}
          onError={(error) => {
            window.settingsError = error
          }}
          onWaterfallChange={() => {}}
        />
      ) : null}
    </main>
  )
}

createRoot(document.getElementById('root')).render(<Fixture />)
