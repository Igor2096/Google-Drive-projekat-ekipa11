import { DatePipe } from '@angular/common';
import { Component, ElementRef, HostListener, computed, effect, inject, input, signal, viewChild } from '@angular/core';
import { takeUntilDestroyed, toObservable } from '@angular/core/rxjs-interop';
import { Router, RouterLink } from '@angular/router';
import { Observable, catchError, of, switchMap, tap } from 'rxjs';
import { DriveFile, Folder, FolderContents } from '../../core/models/drive.models';
import { DriveStateService, errorMessage } from '../../core/services/drive-state.service';
import { FolderService } from '../../core/services/folder.service';
import { FileService, validateUpload } from '../../core/services/file.service';
import { Icon } from '../../shared/components/icon/icon';
import { NameDialog } from '../../shared/components/name-dialog/name-dialog';
import { FileSizePipe } from '../../shared/pipes/file-size.pipe';

type ViewMode = 'grid' | 'list';
type ContextMenu =
  | { kind: 'folder'; folder: Folder; x: number; y: number }
  | { kind: 'file'; file: DriveFile; x: number; y: number };

@Component({
  selector: 'app-drive-page',
  imports: [RouterLink, DatePipe, Icon, NameDialog, FileSizePipe],
  templateUrl: './drive-page.html',
})
export class DrivePage {
  /** Parametar iz rute /folders/:id (undefined na korenu). */
  readonly id = input<string>();

  private readonly folderService = inject(FolderService);
  private readonly fileService = inject(FileService);
  private readonly router = inject(Router);
  protected readonly state = inject(DriveStateService);

  private readonly fileInput = viewChild.required<ElementRef<HTMLInputElement>>('fileInput');
  /** null = nova datoteka; inace = nova verzija te datoteke */
  private versionTarget: DriveFile | null = null;

  protected readonly contents = signal<FolderContents | null>(null);
  protected readonly loading = signal(true);
  protected readonly loadError = signal<string | null>(null);
  protected readonly view = signal<ViewMode>(readViewMode());
  protected readonly selectedId = signal<string | null>(null);
  protected readonly menu = signal<ContextMenu | null>(null);

  /** Preimenovanje: folder ili datoteka (1.6) */
  protected readonly renaming = signal<{ kind: 'folder' | 'file'; id: string; name: string } | null>(null);
  protected readonly renameBusy = signal(false);
  protected readonly renameError = signal<string | null>(null);

  protected readonly details = signal<DriveFile | null>(null);
  protected readonly busy = signal(false);

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

  // --- navigacija i izbor ---

  protected open(folder: Folder) {
    this.router.navigate(['/folders', folder.id]);
  }

  protected select(id: string, event: MouseEvent) {
    event.stopPropagation();
    this.selectedId.set(id);
  }

  protected openFolderMenu(folder: Folder, event: MouseEvent) {
    event.preventDefault();
    event.stopPropagation();
    this.selectedId.set(folder.id);
    this.menu.set({ kind: 'folder', folder, x: event.clientX, y: event.clientY });
  }

  protected openFileMenu(file: DriveFile, event: MouseEvent) {
    event.preventDefault();
    event.stopPropagation();
    this.selectedId.set(file.id);
    this.menu.set({ kind: 'file', file, x: event.clientX, y: event.clientY });
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

  // --- preimenovanje foldera i datoteka (1.6) ---

  protected startRenameFolder(folder: Folder) {
    this.menu.set(null);
    this.renameError.set(null);
    this.renaming.set({ kind: 'folder', id: folder.id, name: folder.name });
  }

  protected startRenameFile(file: DriveFile) {
    this.menu.set(null);
    this.renameError.set(null);
    this.renaming.set({ kind: 'file', id: file.id, name: file.name });
  }

  protected confirmRename(name: string) {
    const target = this.renaming();
    if (!target) return;
    if (name === target.name) {
      this.renaming.set(null);
      return;
    }
    this.renameBusy.set(true);
    const request: Observable<{ name: string }> =
      target.kind === 'folder'
        ? this.folderService.rename(target.id, name)
        : this.fileService.rename(target.id, name);
    request.subscribe({
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

  // --- otpremanje i nova verzija (1.5) ---

  /** Prazan folder: dugme "Otpremi datoteku" */
  protected chooseNewFile() {
    this.versionTarget = null;
    this.fileInput().nativeElement.click();
  }

  protected chooseNewVersion(file: DriveFile) {
    this.menu.set(null);
    this.versionTarget = file;
    // Poziva se direktno iz klika - inace browser blokira prozor za izbor fajla.
    this.fileInput().nativeElement.click();
  }

  protected onFileChosen(event: Event) {
    const input = event.target as HTMLInputElement;
    const chosen = input.files?.[0];
    input.value = '';
    if (!chosen) return;

    const problem = validateUpload(chosen);
    if (problem) {
      this.state.showToast(problem);
      return;
    }

    const target = this.versionTarget;
    this.busy.set(true);
    const request = target
      ? this.fileService.uploadNewVersion(target.id, chosen)
      : this.fileService.upload(chosen, this.state.currentFolderId());

    request.subscribe({
      next: (file) => {
        this.busy.set(false);
        this.state.showToast(
          target
            ? `Otpremljena je verzija ${file.version} datoteke „${file.name}".`
            : `Datoteka „${file.name}" je otpremljena.`,
        );
        this.state.requestReload();
      },
      error: (err) => {
        this.busy.set(false);
        this.state.showToast(`Otpremanje nije uspelo: ${errorMessage(err)}`);
      },
    });
  }

  // --- integritet i detalji (2.18) ---

  protected verify(file: DriveFile) {
    this.menu.set(null);
    this.fileService.verify(file.id).subscribe({
      next: (result) =>
        this.state.showToast(
          result.valid
            ? `Integritet potvrđen: „${file.name}" nije menjana.`
            : `UPOZORENJE: sadržaj „${file.name}" se ne poklapa sa zabeleženim otiskom!`,
        ),
      error: (err) => this.state.showToast(`Provera nije uspela: ${errorMessage(err)}`),
    });
  }

  protected showDetails(file: DriveFile) {
    this.menu.set(null);
    this.details.set(file);
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
