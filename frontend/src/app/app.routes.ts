import { Routes } from '@angular/router';

export const routes: Routes = [
  {
    path: '',
    pathMatch: 'full',
    loadComponent: () => import('./features/drive/drive-page').then((m) => m.DrivePage),
  },
  {
    path: 'folders/:id',
    loadComponent: () => import('./features/drive/drive-page').then((m) => m.DrivePage),
  },
  { path: '**', redirectTo: '' },
];
