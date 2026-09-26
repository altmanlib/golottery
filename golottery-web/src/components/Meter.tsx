import { Box } from '@mantine/core'
import classes from './Meter.module.css'

/** A thin progress bar; `value` and `max` are counts. */
export function Meter({ value, max, label }: { value: number; max: number; label: string }) {
  const percent = max > 0 ? Math.min(100, (value / max) * 100) : 0
  return (
    <Box className={classes.track} role="meter" aria-label={label} aria-valuenow={value} aria-valuemin={0} aria-valuemax={max} flex={1}>
      <Box className={classes.fill} w={`${percent}%`} />
    </Box>
  )
}
