import JavIdolCoverModal from '@/components/JavIdolCoverModal'
import { JavIdolEditModal } from '@/components/JavIdolGrid'
import { JavEditModal } from '@/features/jav/components/JavEditModal'
import { JavCustomTagModal } from '@/features/jav/components/JavCustomTagModal'

export function JavItemEditors({
  idolCoverEditorItem,

  preferChineseName,
  setIdolCoverEditorItem,
  handleIdolCoverSaved,
  idolEditorItem,
  setIdolEditorItem,
  handleIdolSaved,
  setPreviewIdol,
  editorOpen,
  item,
  setEditorOpen,
  handleEditorSaved,
  customTagEditorOpen,
  setCustomTagEditorOpen,
  handleCustomTagsSaved,
}) {
  return (
    <>
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
