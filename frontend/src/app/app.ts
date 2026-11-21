import { Component, HostListener, inject, signal } from '@angular/core';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { environment } from '../environments/environment';
import { DriveStateService, errorMessage } from './core/services/drive-state.service';
import { FolderService } from './core/services/folder.service';
import { Icon } from './shared/components/icon/icon';
import { NameDialog } from './shared/components/name-dialog/name-dialog';

@Component({
  selector: 'app-root',
  imports: [RouterOutlet, RouterLink, RouterLinkActive, Icon, NameDialog],
  templateUrl: './app.html',
})
export class App {
  protected readonly state = inject(DriveStateService);
  private readonly folders = inject(FolderService);

  protected readonly userInitials = environment.devUserId.slice(0, 2).toUpperCase();
  protected readonly newMenuOpen = signal(false);
  protected readonly creating = signal(false);
  protected readonly createError = signal<string | null>(null);

  protected toggleNewMenu(event: MouseEvent) {
    event.stopPropagation();
    this.newMenuOpen.update((v) => !v);
  }

  @HostListener('document:click')
  protected closeMenus() {
    this.newMenuOpen.set(false);
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
}
