import { Stack, Box, Button, Typography } from '@mui/material'
import { Plus } from 'lucide-react'

interface EditableListProps<T> {
  items: T[]
  renderItem: (item: T, index: number, onUpdate: (patch: Partial<T>) => void, onRemove: () => void) => React.ReactNode
  onAdd: () => void
  onRemove: (index: number) => void
  onUpdate: (index: number, patch: Partial<T>) => void
  addButtonLabel: string
  emptyMessage?: string
  maxItems?: number
}

export default function EditableList<T>({
  items,
  renderItem,
  onAdd,
  onRemove,
  onUpdate,
  addButtonLabel,
  emptyMessage = 'Нет элементов',
  maxItems,
}: EditableListProps<T>) {
  if (items.length === 0) {
    return (
      <Box>
        <Typography variant="body2" color="text.secondary">{emptyMessage}</Typography>
        <Button size="small" startIcon={<Plus size={14} />} onClick={onAdd} sx={{ mt: 1 }}>
          {addButtonLabel}
        </Button>
      </Box>
    )
  }

  return (
    <Stack spacing={1.5}>
      {items.map((item, index) => (
        <Box key={index} sx={{ width: '100%' }}>
          {renderItem(
            item,
            index,
            (patch) => onUpdate(index, patch),
            () => onRemove(index)
          )}
        </Box>
      ))}
      {!maxItems || items.length < maxItems ? (
        <Button size="small" startIcon={<Plus size={14} />} onClick={onAdd} sx={{ alignSelf: 'flex-start' }}>
          {addButtonLabel}
        </Button>
      ) : (
        <Typography variant="caption" color="text.secondary">
          Достигнут лимит элементов ({maxItems})
        </Typography>
      )}
    </Stack>
  )
}