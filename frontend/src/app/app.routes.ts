
import { Routes } from '@angular/router';

export const routes: Routes = [
  {
    path: '',
    loadComponent: () =>
      import('./layouts/drive-layout/drive-layout').then(
        (m) => m.DriveLayout
      ),
    children: [
      {
        path: '',
        pathMatch: 'full',
        loadComponent: () =>
          import('./features/drive/drive-page').then((m) => m.DrivePage),
      },
      {
        path: 'folders/:id',
        loadComponent: () =>
          import('./features/drive/drive-page').then((m) => m.DrivePage),
      },
    ],
  },
  {
    path: '',
    loadComponent: () =>
      import('./layouts/auth-layout/auth-layout').then(
        (m) => m.AuthLayout
      ),
    children: [
      {
        path: 'login',
        loadComponent: () =>
          import('./features/auth/login/login').then((m) => m.Login),
      },
      {
        path: 'register',
        loadComponent: () =>
          import('./features/auth/register/register').then((m) => m.Register),
      },
      {
        path: 'verify-otp',
        loadComponent: () =>
          import('./features/auth/verify-otp/verify-otp').then(
            (m) => m.VerifyOtp
          ),
      },
    ],
  },
  {
    path: '**',
    redirectTo: 'login',
  },
];
