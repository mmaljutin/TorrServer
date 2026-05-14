import CssBaseline from '@material-ui/core/CssBaseline'
import { createContext, useCallback, useEffect, useState } from 'react'
import Typography from '@material-ui/core/Typography'
import {
  Menu as MenuIcon,
  Close as CloseIcon,
  Brightness4 as Brightness4Icon,
  Brightness5 as Brightness5Icon,
  BrightnessAuto as BrightnessAutoIcon,
  Sort as SortIcon,
  SortByAlpha as SortByAlphaIcon,
  Search as SearchIcon,
} from '@material-ui/icons'
import { echoHost, streamHost } from 'utils/Hosts'
import Div100vh from 'react-div-100vh'
import axios from 'axios'
import TorrentList from 'components/TorrentList'
import DonateSnackbar from 'components/Donate'
import DonateDialog from 'components/Donate/DonateDialog'
import useChangeLanguage from 'utils/useChangeLanguage'
import { ThemeProvider as MuiThemeProvider } from '@material-ui/core/styles'
import { ThemeProvider as StyledComponentsThemeProvider } from 'styled-components'
import { useQuery } from 'react-query'
import { getTorrents, isStandaloneApp } from 'utils/Utils'
import { isFilePlayable } from 'components/DialogTorrentDetailsContent/helpers'
import GlobalStyle from 'style/GlobalStyle'
import { /* lightTheme, */ THEME_MODES, useMaterialUITheme } from 'style/materialUISetup'
import getStyledComponentsTheme from 'style/getStyledComponentsTheme'
import checkIsIOS from 'utils/checkIsIOS'
import SearchDialog from 'components/Search/SearchDialog'

import { AppWrapper, AppHeader, HeaderToggle, StyledIconButton, SidebarOverlay, SelectionBar, SelectionBarButton } from './style'
import Sidebar from './Sidebar'
import PWAFooter from './PWAFooter'
import { PWAInstallationGuide } from './PWAInstallationGuide'

const snackbarIsClosed = JSON.parse(localStorage.getItem('snackbarIsClosed'))

export const DarkModeContext = createContext()

export default function App() {
  const [isDrawerOpen, setIsDrawerOpen] = useState(false)
  const [isDonationDialogOpen, setIsDonationDialogOpen] = useState(false)
  const [isSearchDialogOpen, setIsSearchDialogOpen] = useState(false)
  const [torrServerVersion, setTorrServerVersion] = useState('')

  const [isDarkMode, currentThemeMode, updateThemeMode, muiTheme] = useMaterialUITheme()
  const [currentLang, changeLang] = useChangeLanguage()
  const [isOffline, setIsOffline] = useState(false)
  const [globalCategoryFilter, setGlobalFilterCategory] = useState('all')
  const { data: torrents, isLoading } = useQuery('torrents', getTorrents, {
    retry: 1,
    refetchInterval: 1000,
    onError: () => setIsOffline(true),
    onSuccess: () => setIsOffline(false),
  })
  const [sortABC, setSortABC] = useState(false)
  const handleClickSortABC = () => setSortABC(true)
  const handleClickSortDate = () => setSortABC(false)

  const [selectedHashes, setSelectedHashes] = useState(new Set())
  const toggleSelect = useCallback(hash => {
    setSelectedHashes(prev => {
      const next = new Set(prev)
      next.has(hash) ? next.delete(hash) : next.add(hash)
      return next
    })
  }, [])
  const clearSelection = () => setSelectedHashes(new Set())

  const downloadPlaylist = () => {
    if (!torrents || selectedHashes.size === 0) return
    const selected = torrents.filter(t => selectedHashes.has(t.hash))
    let m3u = '#EXTM3U\n'
    for (const torrent of selected) {
      const files = (torrent.data && JSON.parse(torrent.data).TorrServer?.Files) || []
      const playable = files.filter(f => isFilePlayable(f.path))
      for (const file of playable) {
        const name = file.path.split('/').pop().split('\\').pop()
        m3u += `#EXTINF:-1,${name}\n`
        m3u += `${streamHost()}/${encodeURIComponent(name)}?link=${torrent.hash}&index=${file.id}&play\n`
      }
    }
    const blob = new Blob([m3u], { type: 'application/x-mpegurl' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'playlist.m3u'
    a.click()
    URL.revokeObjectURL(url)
  }

  useEffect(() => {
    axios.get(echoHost()).then(({ data }) => setTorrServerVersion(data))
  }, [])

  return (
    <>
      <GlobalStyle />

      <DarkModeContext.Provider value={{ isDarkMode }}>
        <MuiThemeProvider theme={muiTheme}>
          <StyledComponentsThemeProvider
            theme={getStyledComponentsTheme(isDarkMode ? THEME_MODES.DARK : THEME_MODES.LIGHT)}
          >
            <CssBaseline />

            {/* Div100vh - iOS WebKit fix  */}
            <Div100vh>
              <AppWrapper isDrawerOpen={isDrawerOpen}>
                <AppHeader>
                  <StyledIconButton edge='start' color='inherit' onClick={() => setIsDrawerOpen(!isDrawerOpen)}>
                    {isDrawerOpen ? <CloseIcon /> : <MenuIcon />}
                  </StyledIconButton>

                  <Typography variant='h6' noWrap>
                    TorrServer {torrServerVersion}
                  </Typography>

                  <div
                    style={{
                      justifySelf: 'end',
                      display: 'grid',
                      gridTemplateColumns: isStandaloneApp ? 'repeat(4, 1fr)' : 'repeat(3, 1fr)',
                      gap: '10px',
                    }}
                  >
                    {isStandaloneApp && (
                      <HeaderToggle onClick={() => setIsSearchDialogOpen(true)}>
                        <SearchIcon />
                      </HeaderToggle>
                    )}

                    <HeaderToggle onClick={() => (sortABC === true ? handleClickSortDate() : handleClickSortABC())}>
                      {sortABC === true ? <SortByAlphaIcon /> : <SortIcon />}
                    </HeaderToggle>

                    <HeaderToggle
                      onClick={() => {
                        if (currentThemeMode === THEME_MODES.LIGHT) updateThemeMode(THEME_MODES.DARK)
                        if (currentThemeMode === THEME_MODES.DARK) updateThemeMode(THEME_MODES.AUTO)
                        if (currentThemeMode === THEME_MODES.AUTO) updateThemeMode(THEME_MODES.LIGHT)
                      }}
                    >
                      {currentThemeMode === THEME_MODES.LIGHT ? (
                        <Brightness5Icon />
                      ) : currentThemeMode === THEME_MODES.DARK ? (
                        <Brightness4Icon />
                      ) : (
                        <BrightnessAutoIcon />
                      )}
                    </HeaderToggle>

                    <HeaderToggle
                      onClick={() =>
                        currentLang === 'en'
                          ? changeLang('ru')
                          : currentLang === 'ru'
                          ? changeLang('ua')
                          : currentLang === 'ua'
                          ? changeLang('zh')
                          : currentLang === 'zh'
                          ? changeLang('bg')
                          : currentLang === 'bg'
                          ? changeLang('fr')
                          : currentLang === 'fr'
                          ? changeLang('ro')
                          : changeLang('en')
                      }
                    >
                      {currentLang.toUpperCase()}
                    </HeaderToggle>
                  </div>
                </AppHeader>

                <SidebarOverlay isDrawerOpen={isDrawerOpen} onClick={() => setIsDrawerOpen(false)} />

                <Sidebar
                  isOffline={isOffline}
                  isLoading={isLoading}
                  isDrawerOpen={isDrawerOpen}
                  setIsDonationDialogOpen={setIsDonationDialogOpen}
                  setGlobalFilterCategory={setGlobalFilterCategory}
                />

                <TorrentList
                  isOffline={isOffline}
                  torrents={torrents}
                  isLoading={isLoading}
                  sortABC={sortABC}
                  sortCategory={globalCategoryFilter}
                  selectedHashes={selectedHashes}
                  onToggleSelect={toggleSelect}
                />

                {selectedHashes.size > 0 && (
                  <SelectionBar>
                    <span>{selectedHashes.size} выбрано</span>
                    <SelectionBarButton primary onClick={downloadPlaylist}>
                      ⬇ Скачать плейлист (.m3u)
                    </SelectionBarButton>
                    <SelectionBarButton onClick={clearSelection}>Отмена</SelectionBarButton>
                  </SelectionBar>
                )}

                <PWAFooter
                  isOffline={isOffline}
                  isLoading={isLoading}
                  setIsDonationDialogOpen={setIsDonationDialogOpen}
                />

                {/* <MuiThemeProvider theme={lightTheme}> */}
                {isDonationDialogOpen && <DonateDialog onClose={() => setIsDonationDialogOpen(false)} />}
                {/* </MuiThemeProvider> */}

                {isSearchDialogOpen && <SearchDialog handleClose={() => setIsSearchDialogOpen(false)} />}

                {snackbarIsClosed ? checkIsIOS() && !isStandaloneApp && <PWAInstallationGuide /> : <DonateSnackbar />}
              </AppWrapper>
            </Div100vh>
          </StyledComponentsThemeProvider>
        </MuiThemeProvider>
      </DarkModeContext.Provider>
    </>
  )
}
