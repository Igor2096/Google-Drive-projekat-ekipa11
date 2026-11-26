
import { Routes } from '@angular/router';
import { adminGuard } from './core/guards/admin.guard';

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
      {
        path: 'quota-request',
        loadComponent: () =>
          import('./features/quota/quota-request/quota-request').then(
            (m) => m.QuotaRequest
          ),
      },
      {
        path: 'admin',
        canActivate: [adminGuard],
        loadComponent: () =>
          import('./features/admin/admin-page/admin-page').then(
      (m) => m.AdminPage
    ),
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
