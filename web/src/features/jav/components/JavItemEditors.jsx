import JavIdolCoverModal from '@/features/jav/components/JavIdolCoverModal'
import { JavIdolEditModal } from '@/features/jav/components/JavIdolGrid'
import { JavEditModal } from '@/features/jav/components/JavEditModal'
import { JavCustomTagModal } from '@/features/jav/components/JavCustomTagModal'
import JavSeriesTranslationModal from '@/features/jav/components/JavSeriesTranslationModal'

export function JavItemEditors({
  idolCoverEditorItem,

  preferChineseName,
  setIdolCoverEditorItem,
  handleIdolCoverSaved,
  idolEditorItem,
  setIdolEditorItem,
  handleIdolSaved,
  seriesEditorItem,
  setSeriesEditorItem,
  handleSeriesSaved,
  setPreviewIdol,
  editorOpen,
  item,
  setEditorOpen,
  handleEditorSaved,
  handleEditorDeleted,
  customTagEditorOpen,
  setCustomTagEditorOpen,
  handleCustomTagsSaved,
}) {
  return (
    <>
      {seriesEditorItem && (
        <JavSeriesTranslationModal
          item={seriesEditorItem}
          onClose={() => setSeriesEditorItem(null)}
          onSaved={handleSeriesSaved}
        />
      )}
      <JavIdolCoverModal
        key={`idol-cover-${idolCoverEditorItem?.id || 'closed'}`}
        open={Boolean(idolCoverEditorItem)}
        item={idolCoverEditorItem}
        preferChineseName={preferChineseName}
        onClose={() => setIdolCoverEditorItem(null)}
        onSaved={handleIdolCoverSaved}
      />
      <JavIdolEditModal
        key={`idol-editor-${idolEditorItem?.id || 'closed'}`}
        open={Boolean(idolEditorItem)}
        item={idolEditorItem}
        preferChineseName={preferChineseName}
        onClose={() => setIdolEditorItem(null)}
        onSaved={handleIdolSaved}
        onMerged={() => {
          setIdolEditorItem(null)
          setPreviewIdol(null)
        }}
      />
      <JavEditModal
        open={editorOpen}
        item={item}
        preferChineseName={preferChineseName}
        onClose={() => setEditorOpen(false)}
        onSaved={handleEditorSaved}
        onDeleted={handleEditorDeleted}
      />
      <JavCustomTagModal
        open={customTagEditorOpen}
        item={item}
        onClose={() => setCustomTagEditorOpen(false)}
        onSaved={handleCustomTagsSaved}
      />
    </>
  )
}
