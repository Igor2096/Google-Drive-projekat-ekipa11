import { DatePipe } from '@angular/common';
import { Component, HostListener, computed, effect, inject, input, signal } from '@angular/core';
import { takeUntilDestroyed, toObservable } from '@angular/core/rxjs-interop';
import { Router, RouterLink } from '@angular/router';
import { catchError, of, switchMap, tap } from 'rxjs';
import { DriveFile, Folder, FolderContents } from '../../core/models/drive.models';
import { DriveStateService, errorMessage } from '../../core/services/drive-state.service';
import { FolderService } from '../../core/services/folder.service';
import { Icon } from '../../shared/components/icon/icon';
import { NameDialog } from '../../shared/components/name-dialog/name-dialog';
import { FileSizePipe } from '../../shared/pipes/file-size.pipe';

type ViewMode = 'grid' | 'list';

@Component({
  selector: 'app-drive-page',
  imports: [RouterLink, DatePipe, Icon, NameDialog, FileSizePipe],
  templateUrl: './drive-page.html',
})
export class DrivePage {
  /** Parametar iz rute /folders/:id (undefined na korenu). */
  readonly id = input<string>();

  private readonly folderService = inject(FolderService);
  private readonly router = inject(Router);
  protected readonly state = inject(DriveStateService);

  protected readonly contents = signal<FolderContents | null>(null);
  protected readonly loading = signal(true);
  protected readonly loadError = signal<string | null>(null);
  protected readonly view = signal<ViewMode>(readViewMode());
  protected readonly selectedId = signal<string | null>(null);
  protected readonly menu = signal<{ folder: Folder; x: number; y: number } | null>(null);

  protected readonly renaming = signal<Folder | null>(null);
  protected readonly renameBusy = signal(false);
  protected readonly renameError = signal<string | null>(null);

  protected readonly isEmpty = computed(() => {
    const c = this.contents();
    return !!c && c.folders.length === 0 && c.files.length === 0;
  });

  constructor() {
    // Svaka promena foldera u ruti ili "reload" ponovo ucitava sadrzaj.
    // switchMap otkazuje prethodni zahtev ako korisnik brzo klikce.
    const request = computed(() => ({ id: this.id() ?? null, tick: this.state.reloadTick() }));
    toObservable(request)
      .pipe(
        tap(({ id }) => {
          this.state.currentFolderId.set(id);
          this.loading.set(true);
          this.loadError.set(null);
          this.menu.set(null);
        }),
        switchMap(({ id }) =>
          this.folderService.getContents(id).pipe(
            catchError((err) => {
              this.loadError.set(errorMessage(err));
              return of(null);
            }),
          ),
        ),
        takeUntilDestroyed(),
      )
      .subscribe((contents) => {
        this.contents.set(contents);
        this.loading.set(false);
      });

    effect(() => {
      try {
        localStorage.setItem('drive.view', this.view());
      } catch {
        /* storage nije dostupan - nije bitno */
      }
    });
  }

  protected open(folder: Folder) {
    this.router.navigate(['/folders', folder.id]);
  }

  protected select(folder: Folder, event: MouseEvent) {
    event.stopPropagation();
    this.selectedId.set(folder.id);
  }

  protected openMenu(folder: Folder, event: MouseEvent) {
    event.preventDefault();
    event.stopPropagation();
    this.selectedId.set(folder.id);
    this.menu.set({ folder, x: event.clientX, y: event.clientY });
  }

  @HostListener('document:click')
  protected clearSelection() {
    this.menu.set(null);
    this.selectedId.set(null);
  }

  @HostListener('document:keydown.escape')
  protected closeMenu() {
    this.menu.set(null);
  }

  protected newFolder() {
    this.state.newFolderDialogOpen.set(true);
  }

  protected retry() {
    this.state.requestReload();
  }

  protected startRename(folder: Folder) {
    this.menu.set(null);
    this.renameError.set(null);
    this.renaming.set(folder);
  }

  protected confirmRename(name: string) {
    const folder = this.renaming();
    if (!folder) return;
    if (name === folder.name) {
      this.renaming.set(null);
      return;
    }
    this.renameBusy.set(true);
    this.folderService.rename(folder.id, name).subscribe({
      next: (updated) => {
        this.renameBusy.set(false);
        this.renaming.set(null);
        this.state.showToast(`Preimenovano u „${updated.name}".`);
        this.state.requestReload();
      },
      error: (err) => {
        this.renameBusy.set(false);
        this.renameError.set(errorMessage(err));
      },
    });
  }

  protected fileIcon(file: DriveFile) {
    return file.mimeType.startsWith('image/') ? 'image' : 'file';
  }
}

function readViewMode(): ViewMode {
  try {
    return localStorage.getItem('drive.view') === 'list' ? 'list' : 'grid';
  } catch {
    return 'grid';
  }
}
