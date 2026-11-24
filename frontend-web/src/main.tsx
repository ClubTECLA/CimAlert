import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './app/App.tsx'
import { AuthProvider } from './features/sesion/AuthProvider.tsx'
import { SysMessageProvider } from './components/ui/SysMessageContext.tsx'
import NavigationWrapper from './app/NavigationWrapper.tsx'
import { BrowserRouter } from 'react-router-dom'
import { CommonPanelProvider } from './panels/CommonPanelProvider.tsx'
import { ModalsProvider } from './panels/ModalsProvider.tsx'
import ModalChrome from './panels/ModalChrome.tsx'
import PanelChromeMount from './panels/PanelChromeMount.tsx'
import { MapProvider } from './features/mapa/MapProvider.tsx'
import { LocationPickerProvider } from './features/mapa/useLocationPicker.ts'
import { ReportesProvider } from './features/reportes/ReportesProvider.tsx'
import { RiesgoProvider } from './features/riesgo/RiesgoProvider.tsx'
import { CentrosProvider } from './features/centros/CentrosProvider.tsx'
import { PolygonSelectionProvider } from './features/mapa/PolygonSelectionProvider.tsx'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <SysMessageProvider>
      <AuthProvider>
        <BrowserRouter>
          <ModalsProvider>
            <MapProvider>
              <LocationPickerProvider>
                <ReportesProvider>
                  <CentrosProvider>
                    {/* Por encima de CommonPanelProvider: el panel "Riesgo" se dibuja dentro de él. */}
                    <RiesgoProvider>
                      <CommonPanelProvider>
                        {/* El proveedor del área va dentro del de paneles: al terminar el dibujo abre el panel del área
                            y al empezar cierra el que estuviera abierto para liberar el mapa. */}
                        <PolygonSelectionProvider>
                          {/* Chrome vive en el nivel raíz para superponerse al contenido de navegación. El panel se
                              monta aquí para que sus vistas vean también el contexto del área dibujada. */}
                          <ModalChrome />
                          <PanelChromeMount />
                          <NavigationWrapper />
                          <App />
                        </PolygonSelectionProvider>
                      </CommonPanelProvider>
                    </RiesgoProvider>
                  </CentrosProvider>
                </ReportesProvider>
              </LocationPickerProvider>
            </MapProvider>
          </ModalsProvider>
        </BrowserRouter>
      </AuthProvider>
    </SysMessageProvider>
  </StrictMode>,
)
