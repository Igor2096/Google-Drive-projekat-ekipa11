// Odgovara Go modelima u backend/files-service/internal/model/models.go

export interface Folder {
  id: string;
  name: string;
  ownerId: string;
  parentId: string | null;
  ancestors: string[];
  createdAt: string;
  updatedAt: string;
}

export interface DriveFile {
  id: string;
  name: string;
  ownerId: string;
  folderId: string | null;
  mimeType: string;
  size: number;
  version: number;
  createdAt: string;
  updatedAt: string;
}

export interface Breadcrumb {
  id: string;
  name: string;
}

export interface FolderContents {
  folder: Folder | null;
  breadcrumbs: Breadcrumb[];
  folders: Folder[];
  files: DriveFile[];
}
