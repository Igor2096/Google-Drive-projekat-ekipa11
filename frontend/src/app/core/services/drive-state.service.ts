import { HttpErrorResponse } from '@angular/common/http';
import { Injectable, signal } from '@angular/core';

/** Deljeno UI stanje izmedju layout-a (sidebar "Novo") i stranice sa folderima. */
@Injectable({ providedIn: 'root' })
export class DriveStateService {
  /** Folder koji je trenutno otvoren (null = Moj disk). */
  readonly currentFolderId = signal<string | null>(null);
  /** Povecava se kad sadrzaj treba ponovo ucitati (npr. posle kreiranja foldera). */
  readonly reloadTick = signal(0);
  readonly newFolderDialogOpen = signal(false);
  readonly toast = signal<string | null>(null);

  private toastTimer?: ReturnType<typeof setTimeout>;

  requestReload() {
    this.reloadTick.update((n) => n + 1);
  }

  showToast(message: string) {
    clearTimeout(this.toastTimer);
    this.toast.set(message);
    this.toastTimer = setTimeout(() => this.toast.set(null), 4000);
  }
}

/**
 * Izvlaci poruku iz greske. Files servis vraca {"error": "..."}, a gateway i
 * Users servis obican tekst (npr. "Previse zahteva"), pa se pokrivaju oba.
 */
export function errorMessage(err: unknown): string {
  if (err instanceof HttpErrorResponse) {
    if (err.status === 0) {
      return 'Server nije dostupan. Proveri da li radi i da li je prihvaćen sertifikat (https://localhost:8080/health).';
    }
    const body = err.error as { error?: string } | string | null;
    if (typeof body === 'string' && body.trim()) return body.trim();
    if (body && typeof body === 'object' && typeof body.error === 'string') return body.error;
    return `Greška ${err.status}`;
  }
  return 'Došlo je do neočekivane greške.';
}
