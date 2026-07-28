import { openSteamGuardModal } from "../../stores/modal";
import type {
  SteamGuardAccountRef,
  SteamGuardModalController,
  SteamGuardModalEntry,
} from "../steamGuardModal";
import { onSteamGuardMenuRequest } from "./steamGuardMenuRequest";
import type { SteamGuardMenuRequest } from "./types";

function accountFromRequest(request: SteamGuardMenuRequest): SteamGuardAccountRef {
  return {
    id: request.steamId64,
    username: request.accountName,
    displayName: request.displayName || undefined,
    imageUrl: request.imageUrl || undefined,
    staticImageUrl: request.staticImageUrl || undefined,
  };
}

function entryFromRequest(request: SteamGuardMenuRequest): SteamGuardModalEntry {
  if (request.action === "all") return "all-accounts";
  if (request.action === "add") return "enrollment";
  if (request.action === "import") return "import";
  return "account";
}

export function bindSteamGuardMenuToModal(controller: SteamGuardModalController): () => void {
  return onSteamGuardMenuRequest((request) => {
    void openSteamGuardModal({
      account: accountFromRequest(request),
      controller,
      entry: entryFromRequest(request),
    });
  });
}
