import { Component, ElementRef, HostListener, inject, signal, viewChild } from '@angular/core';
import { Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { DriveStateService, errorMessage } from '../../core/services/drive-state.service';
import { FolderService } from '../../core/services/folder.service';
import { FileService, validateUpload } from '../../core/services/file.service';
import { NotificationService } from '../../core/services/notification.service';
import { AuthService } from '../../core/services/auth.service';
import { Icon } from '../../shared/components/icon/icon';
import { NameDialog } from '../../shared/components/name-dialog/name-dialog';
import { NotificationList } from '../../features/notifications/notification-list';

@Component({
  selector: 'app-drive-layout',
  standalone: true,
  imports: [RouterOutlet, RouterLink, RouterLinkActive, Icon, NameDialog, NotificationList],
  templateUrl: './drive-layout.html',
})
export class DriveLayout {
  protected readonly state = inject(DriveStateService);
  private readonly folders = inject(FolderService);
  private readonly files = inject(FileService);
  private readonly notificationService = inject(NotificationService);
  private readonly auth = inject(AuthService);
  private readonly router = inject(Router);

  private readonly uploadInput = viewChild.required<ElementRef<HTMLInputElement>>('uploadInput');

  protected readonly email = emailFromToken(this.auth.getToken());
  protected readonly userInitials = (this.email || '?').slice(0, 2).toUpperCase();

  protected readonly isAdmin = this.auth.isAdmin();

  protected readonly newMenuOpen = signal(false);
  protected readonly userMenuOpen = signal(false);
  protected readonly creating = signal(false);
  protected readonly createError = signal<string | null>(null);
  protected readonly uploading = signal(false);

  protected readonly notificationsOpen = signal(false);
  protected readonly unreadCount = signal(0);

  /**
   * Stari argument za NotificationService. Novi servis (Igorov paket) ga
   * ignorise i sam odredjuje korisnika iz tokena; prosledjuje se samo da bi
   * ovaj fajl radio bez obzira da li je Igorov paket vec spojen.
   */
  private readonly legacyNotificationUserId = '11111111-1111-1111-1111-111111111111';

  constructor() {
    this.loadUnreadCount();
  }

  protected toggleNewMenu(event: MouseEvent) {
    event.stopPropagation();
    this.newMenuOpen.update((v) => !v);
  }

  protected toggleUserMenu(event: MouseEvent) {
    event.stopPropagation();
    this.userMenuOpen.update((v) => !v);
  }

  @HostListener('document:click')
  protected closeMenus() {
    this.newMenuOpen.set(false);
    this.userMenuOpen.set(false);
    this.notificationsOpen.set(false);
  }

  protected openNewFolder() {
    this.newMenuOpen.set(false);
    this.createError.set(null);
    this.state.newFolderDialogOpen.set(true);
  }

  protected closeNewFolder() {
    this.state.newFolderDialogOpen.set(false);
  }

  protected createFolder(name: string) {
    this.creating.set(true);
    this.createError.set(null);

    this.folders.create(name, this.state.currentFolderId()).subscribe({
      next: (folder) => {
        this.creating.set(false);
        this.state.newFolderDialogOpen.set(false);
        this.state.showToast(`Folder „${folder.name}" je napravljen.`);
        this.state.requestReload();
      },
      error: (err) => {
        this.creating.set(false);
        this.createError.set(errorMessage(err));
      },
    });
  }

  // --- otpremanje (1.5) ---

  /** Klik na "Otpremanje datoteke" otvara sistemski izbor fajla. */
  protected chooseFile() {
    this.newMenuOpen.set(false);
    // Mora da se pozove direktno iz klika, inace browser blokira prozor za izbor.
    this.uploadInput().nativeElement.click();
  }

  protected onFileChosen(event: Event) {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    input.value = ''; // da isti fajl moze ponovo da se izabere
    if (!file) return;

    const problem = validateUpload(file);
    if (problem) {
      this.state.showToast(problem);
      return;
    }

    this.uploading.set(true);
    this.state.showToast(`Otpremam „${file.name}"...`);
    this.files.upload(file, this.state.currentFolderId()).subscribe({
      next: (uploaded) => {
        this.uploading.set(false);
        this.state.showToast(`Datoteka „${uploaded.name}" je otpremljena.`);
        this.state.requestReload();
      },
      error: (err) => {
        this.uploading.set(false);
        this.state.showToast(`Otpremanje nije uspelo: ${errorMessage(err)}`);
      },
    });
  }

  // --- odjava (1.2) ---

  protected logout() {
    sessionStorage.removeItem('accessToken');
    this.router.navigate(['/login']);
  }

  // --- notifikacije ---

  protected toggleNotifications(event: MouseEvent) {
    event.stopPropagation();
    this.notificationsOpen.update((value) => !value);
  }

  protected loadUnreadCount() {
    this.notificationService.getNotifications(this.legacyNotificationUserId).subscribe({
      next: (notifications) => {
        this.unreadCount.set((notifications ?? []).filter((n) => !n.is_read).length);
      },
      error: (err) => console.error('Greška pri učitavanju obaveštenja:', err),
    });
  }
}

/** Mejl iz JWT-a, samo za prikaz. Token je potpisan; ovde se NE proverava. */
function emailFromToken(token: string | null): string {
  try {
    const payload = token?.split('.')[1];
    if (!payload) return '';
    const json = atob(payload.replace(/-/g, '+').replace(/_/g, '/'));
    return JSON.parse(json).email ?? '';
  } catch {
    return '';
  }
}
