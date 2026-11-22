import UserMenu from "@/features/perfil/views/UserMenu";
import NotificationsMenu from "@/features/notificaciones/views/NotificationsMenu";
import DeleteIncidentConfirmation from "@/features/ajustes/components/DeleteIncidentConfirmation";
import DiscardChangesConfirmation from "@/features/perfil/components/DiscardChangesConfirmation";

// Se guardan componentes para que ModalChrome los monte con las props de cada apertura.
export const modalRegistry = {
    "Menu de Usuario": UserMenu,
    "Menu de Notificaciones": NotificationsMenu,
    "Confirmar eliminación de incidente": DeleteIncidentConfirmation,
    "Confirmar descarte de cambios": DiscardChangesConfirmation,
} as const;
