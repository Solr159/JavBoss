import { useState } from 'react'
import { createDisplaySettingsDraft } from '@/features/settings/model'
import { useStore } from '@/store'
import { saveJavSettings, saveVideoSettings } from '@/features/settings/actions'
import { getErrorMessage } from '@/utils/errors'
import DisplaySettingsModal from '@/features/settings/components/DisplaySettingsModal'

export default function DisplaySettings({ onClose, onError, onWaterfallChange, initialTab }) {
  const [draft, setDraft] = useState(() => createDisplaySettingsDraft(useStore.getState()))
  const [editingTabs, setEditingTabs] = useState({})
  const [savingTabs, setSavingTabs] = useState({})
  const resetTabDraft = (tab) => {
    const saved = createDisplaySettingsDraft(useStore.getState(), tab)
    setDraft((current) => ({ ...current, ...saved }))
  }
  const handleSave = async (tab) => {
    if (savingTabs[tab]) return
    setSavingTabs((current) => ({ ...current, [tab]: true }))
    try {
      if (tab === 'video') await saveVideoSettings(draft, onWaterfallChange)
      else await saveJavSettings(draft, onWaterfallChange, tab)
      onClose()
    } catch (error) {
      onError(getErrorMessage(error))
    } finally {
      setSavingTabs((current) => ({ ...current, [tab]: false }))
    }
  }
  return (
    <DisplaySettingsModal
      open={true}
      initialTab={initialTab}
      onClose={() => onClose()}
      editingTabs={editingTabs}
      savingTabs={savingTabs}
      onEdit={(tab) => {
        resetTabDraft(tab)
        setEditingTabs((current) => ({ ...current, [tab]: true }))
      }}
      onCancel={(tab) => {
        resetTabDraft(tab)
        setEditingTabs((current) => ({ ...current, [tab]: false }))
      }}
      videoSettings={{
        pageSizeInput: draft.videoPageSizeInput,
        onPageSizeChange: (value) =>
          setDraft((current) => ({ ...current, videoPageSizeInput: value })),
        sortInput: draft.videoSortInput,
        onSortChange: (value) => setDraft((current) => ({ ...current, videoSortInput: value })),
        hideJavInput: draft.videoHideJavInput,
        onHideJavChange: (value) =>
          setDraft((current) => ({ ...current, videoHideJavInput: value })),
        waterfallDefaultInput: draft.videoWaterfallDefaultInput,
        onWaterfallDefaultChange: (value) =>
          setDraft((current) => ({ ...current, videoWaterfallDefaultInput: value })),
        watchTimeIconMinutesInput: draft.videoWatchTimeIconMinutesInput,
        onWatchTimeIconMinutesChange: (value) =>
          setDraft((current) => ({ ...current, videoWatchTimeIconMinutesInput: value })),
        autoSubtitleEnabledInput: draft.autoSubtitleEnabledInput,
        onAutoSubtitleEnabledChange: (value) =>
          setDraft((current) => ({ ...current, autoSubtitleEnabledInput: value })),
        autoSubtitleLanguageInput: draft.autoSubtitleLanguageInput,
        onAutoSubtitleLanguageChange: (value) =>
          setDraft((current) => ({ ...current, autoSubtitleLanguageInput: value })),
      }}
      javPageSizeInput={draft.javPageSizeInput}
      onJavPageSizeChange={(value) =>
        setDraft((current) => ({ ...current, javPageSizeInput: value }))
      }
      javGridColumnsInput={draft.javGridColumnsInput}
      javCardPortraitInput={draft.javCardPortraitInput}
      onJavCardPortraitChange={(value) =>
        setDraft((current) => ({ ...current, javCardPortraitInput: value }))
      }
      onJavGridColumnsChange={(value) =>
        setDraft((current) => ({ ...current, javGridColumnsInput: value }))
      }
      javTitleMaxRowsInput={draft.javTitleMaxRowsInput}
      onJavTitleMaxRowsChange={(value) =>
        setDraft((current) => ({ ...current, javTitleMaxRowsInput: value }))
      }
      javIdolTagMaxRowsInput={draft.javIdolTagMaxRowsInput}
      onJavIdolTagMaxRowsChange={(value) =>
        setDraft((current) => ({ ...current, javIdolTagMaxRowsInput: value }))
      }
      javTagMaxRowsInput={draft.javTagMaxRowsInput}
      onJavTagMaxRowsChange={(value) =>
        setDraft((current) => ({ ...current, javTagMaxRowsInput: value }))
      }
      javHideStudioInput={draft.javHideStudioInput}
      onJavHideStudioChange={(value) =>
        setDraft((current) => ({ ...current, javHideStudioInput: value }))
      }
      javHideSeriesInput={draft.javHideSeriesInput}
      onJavHideSeriesChange={(value) =>
        setDraft((current) => ({ ...current, javHideSeriesInput: value }))
      }
      javHideIdolsInput={draft.javHideIdolsInput}
      onJavHideIdolsChange={(value) =>
        setDraft((current) => ({ ...current, javHideIdolsInput: value }))
      }
      javHideTagsInput={draft.javHideTagsInput}
      onJavHideTagsChange={(value) =>
        setDraft((current) => ({ ...current, javHideTagsInput: value }))
      }
      javHideActionsInput={draft.javHideActionsInput}
      onJavHideActionsChange={(value) =>
        setDraft((current) => ({ ...current, javHideActionsInput: value }))
      }
      javFavoriteRatingShowFullInput={draft.javFavoriteRatingShowFullInput}
      onJavFavoriteRatingShowFullChange={(value) =>
        setDraft((current) => ({ ...current, javFavoriteRatingShowFullInput: value }))
      }
      javWaterfallDefaultInput={draft.javWaterfallDefaultInput}
      onJavWaterfallDefaultChange={(value) =>
        setDraft((current) => ({ ...current, javWaterfallDefaultInput: value }))
      }
      idolPageSizeInput={draft.idolPageSizeInput}
      onIdolPageSizeChange={(value) =>
        setDraft((current) => ({ ...current, idolPageSizeInput: value }))
      }
      idolWaterfallDefaultInput={draft.idolWaterfallDefaultInput}
      onIdolWaterfallDefaultChange={(value) =>
        setDraft((current) => ({ ...current, idolWaterfallDefaultInput: value }))
      }
      studioPageSizeInput={draft.studioPageSizeInput}
      onStudioPageSizeChange={(value) =>
        setDraft((current) => ({ ...current, studioPageSizeInput: value }))
      }
      studioWaterfallDefaultInput={draft.studioWaterfallDefaultInput}
      onStudioWaterfallDefaultChange={(value) =>
        setDraft((current) => ({ ...current, studioWaterfallDefaultInput: value }))
      }
      seriesPageSizeInput={draft.seriesPageSizeInput}
      onSeriesPageSizeChange={(value) =>
        setDraft((current) => ({ ...current, seriesPageSizeInput: value }))
      }
      seriesWaterfallDefaultInput={draft.seriesWaterfallDefaultInput}
      onSeriesWaterfallDefaultChange={(value) =>
        setDraft((current) => ({ ...current, seriesWaterfallDefaultInput: value }))
      }
      javSortInput={draft.javSortInput}
      onJavSortChange={(value) => setDraft((current) => ({ ...current, javSortInput: value }))}
      javSortRulesInput={draft.javSortRulesInput}
      onJavSortRulesChange={(value) =>
        setDraft((current) => ({ ...current, javSortRulesInput: value }))
      }
      idolSortInput={draft.idolSortInput}
      onIdolSortChange={(value) => setDraft((current) => ({ ...current, idolSortInput: value }))}
      javIdolPreferChineseNameInput={draft.javIdolPreferChineseNameInput}
      onJavIdolPreferChineseNameChange={(value) =>
        setDraft((current) => ({ ...current, javIdolPreferChineseNameInput: value }))
      }
      javTagShowSimplifiedInput={draft.javTagShowSimplifiedInput}
      onJavTagShowSimplifiedChange={(value) =>
        setDraft((current) => ({ ...current, javTagShowSimplifiedInput: value }))
      }
      javWatchTimeIconMinutesInput={draft.javWatchTimeIconMinutesInput}
      onJavWatchTimeIconMinutesChange={(value) =>
        setDraft((current) => ({ ...current, javWatchTimeIconMinutesInput: value }))
      }
      onSave={handleSave}
    />
  )
}
