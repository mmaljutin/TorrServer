import { Button, Dialog, DialogActions, DialogContent, DialogContentText, DialogTitle } from '@material-ui/core'
import ListItem from '@material-ui/core/ListItem'
import ListItemIcon from '@material-ui/core/ListItemIcon'
import ListItemText from '@material-ui/core/ListItemText'
import CleaningServicesIcon from '@material-ui/icons/DeleteSweep'
import { useState } from 'react'
import { cacheHost } from 'utils/Hosts'
import { useTranslation } from 'react-i18next'

import UnsafeButton from './UnsafeButton'

const fnClearAllCache = () =>
  fetch(cacheHost(), {
    method: 'post',
    body: JSON.stringify({ action: 'clearall' }),
    headers: {
      Accept: 'application/json, text/plain, */*',
      'Content-Type': 'application/json',
    },
  })

export default function ClearCache({ isOffline, isLoading }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const closeDialog = () => setOpen(false)
  const openDialog = () => setOpen(true)

  return (
    <>
      <ListItem disabled={isOffline || isLoading} button key={t('ClearCache')} onClick={openDialog}>
        <ListItemIcon>
          <CleaningServicesIcon />
        </ListItemIcon>

        <ListItemText primary={t('ClearCache')} />
      </ListItem>

      <Dialog open={open} onClose={closeDialog}>
        <DialogTitle>{t('ClearCache?')}</DialogTitle>
        <DialogContent>
          <DialogContentText>{t('ClearCacheDesc')}</DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button variant='outlined' onClick={closeDialog} color='secondary'>
            {t('Cancel')}
          </Button>

          <UnsafeButton
            timeout={5}
            startIcon={<CleaningServicesIcon />}
            variant='contained'
            onClick={() => {
              fnClearAllCache()
              closeDialog()
            }}
            color='secondary'
            autoFocus
          >
            {t('OK')}
          </UnsafeButton>
        </DialogActions>
      </Dialog>
    </>
  )
}
