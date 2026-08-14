import TagManagementModal from '@/components/TagManagementModal'
import { isUserJavTag } from '@/constants/jav'
import { zh } from '@/utils/i18n'

const categoryEnglishLabels = {
  默认分类: 'Default',
  主题: 'Theme',
  角色: 'Role',
  服装: 'Clothing',
  体型: 'Body type',
  行为: 'Activity',
  玩法: 'Play',
  类别: 'Type',
  场景: 'Scene',
  其他: 'Other',
}

export default function JavTagModal(props) {
  return (
    <TagManagementModal
      {...props}
      isTagEditable={isUserJavTag}
      tagClassName={(tag) => (isUserJavTag(tag) ? 'skeuo-tag--user' : 'skeuo-tag--scraped')}
      tagLegend={[
        {
          label: zh('我创建的标签', 'My tags'),
          className: 'border-emerald-200 bg-emerald-100',
        },
        {
          label: zh('抓取标签', 'Scraped tags'),
          className: 'border-orange-200 bg-orange-100',
        },
      ]}
      editModeMessage={zh('只可编辑我创建的标签', 'Only my tags can be edited')}
      categoryEnglishLabels={categoryEnglishLabels}
      formatOrganizeResult={(result) =>
        zh(
          `整理完成：从 JavBus 读取 ${result?.remote_tag_count || 0} 个标签，匹配 ${result?.matched_tag_count || 0} 个，更新 ${result?.updated_tag_count || 0} 个，未匹配 ${result?.unmatched_tag_count || 0} 个`,
          `Organized: ${result?.remote_tag_count || 0} read from JavBus, ${result?.matched_tag_count || 0} matched, ${result?.updated_tag_count || 0} updated, ${result?.unmatched_tag_count || 0} unmatched`
        )
      }
      organizeButtonTitle={zh(
        '从 JavBus 读取标签分类进行整理',
        'Read tag categories from JavBus and organize them'
      )}
      organizeConfirmMessage={zh(
        '自动整理将从 JavBus 读取标签分类，并更新所有匹配标签的现有分类。未匹配的标签不受影响。确认继续吗？',
        'Auto organize will read tag categories from JavBus and update the existing category of every matched tag. Unmatched tags will not be changed. Continue?'
      )}
    />
  )
}
