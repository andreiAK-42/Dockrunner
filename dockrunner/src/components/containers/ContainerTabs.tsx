import { Tabs, Tab, Box } from '@mui/material'

interface ContainerTabsProps {
  value: number
  onChange: (event: React.SyntheticEvent, value: number) => void
  tabs?: Array<{ label: string; icon?: React.ReactElement }>
}

export default function ContainerTabs({
  value,
  onChange,
  tabs = [
    { label: 'Обзор' },
    { label: 'Логи' },
    { label: 'Настройки' },
  ],
}: ContainerTabsProps) {
  return (
    <Box>
      <Tabs value={value} onChange={onChange} sx={{ px: 2 }}>
        {tabs.map((tab, index) => (
          <Tab key={index} label={tab.label} icon={tab.icon} />
        ))}
      </Tabs>
    </Box>
  )
}