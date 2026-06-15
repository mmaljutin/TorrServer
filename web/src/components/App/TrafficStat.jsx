import { useEffect, useState } from 'react'
import axios from 'axios'
import { useTranslation } from 'react-i18next'
import { ArrowDownward as ArrowDownwardIcon, ArrowUpward as ArrowUpwardIcon, Cast as CastIcon } from '@material-ui/icons'
import { humanizeSize } from 'utils/Utils'
import { trafficStatHost } from 'utils/Hosts'

// TrafficStat shows server-wide, session-scoped traffic totals: internet down/up
// (summed over active torrents) and bytes served to players over LAN.
const TrafficStat = ({ isOffline }) => {
  const { t } = useTranslation()
  const [stat, setStat] = useState({ download: 0, upload: 0, served: 0 })

  useEffect(() => {
    if (isOffline) return undefined

    let active = true
    const fetchStat = () => {
      axios
        .get(trafficStatHost())
        .then(({ data }) => active && data && setStat(data))
        .catch(() => {})
    }
    fetchStat()
    const id = setInterval(fetchStat, 3000)
    return () => {
      active = false
      clearInterval(id)
    }
  }, [isOffline])

  if (isOffline) return null

  const row = { display: 'flex', alignItems: 'center', gap: 4 }
  const icon = { fontSize: 16, opacity: 0.7 }

  return (
    <div
      title={t('TrafficTotal')}
      style={{ padding: '8px 16px', fontSize: '0.8em', opacity: 0.85, display: 'flex', gap: 12, flexWrap: 'wrap' }}
    >
      <span style={row}>
        <ArrowDownwardIcon style={icon} />
        {humanizeSize(stat.download) || `0 ${t('B')}`}
      </span>
      <span style={row}>
        <ArrowUpwardIcon style={icon} />
        {humanizeSize(stat.upload) || `0 ${t('B')}`}
      </span>
      <span style={row}>
        <CastIcon style={icon} />
        {humanizeSize(stat.served) || `0 ${t('B')}`}
      </span>
    </div>
  )
}

export default TrafficStat
