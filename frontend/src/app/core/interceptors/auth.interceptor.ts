import { HttpErrorResponse, HttpInterceptorFn } from '@angular/common/http';
import { inject } from '@angular/core';
import { Router } from '@angular/router';
import { catchError, throwError } from 'rxjs';

// Ove rute ne traze token - korisnik tada jos nije prijavljen.
const PUBLIC_ENDPOINTS = ['/register', '/login', '/verify-otp'];

export const authInterceptor: HttpInterceptorFn = (req, next) => {
  const router = inject(Router);
  const token = sessionStorage.getItem('accessToken');

  const isPublic = PUBLIC_ENDPOINTS.some((endpoint) => req.url.endsWith(endpoint));
  if (!token || isPublic) {
    return next(req);
  }

  const authRequest = req.clone({
    setHeaders: { Authorization: `Bearer ${token}` },
  });

  return next(authRequest).pipe(
    catchError((error: unknown) => {
      // 401 na zasticenoj ruti znaci da je token istekao ili je neispravan:
      // korisnik se odjavljuje i vraca na prijavu, umesto da UI samo
      // izbacuje greske posle isteka sesije.
      if (error instanceof HttpErrorResponse && error.status === 401) {
        sessionStorage.removeItem('accessToken');
        router.navigate(['/login'], { queryParams: { sesija: 'istekla' } });
      }
      return throwError(() => error);
    }),
  );
};
