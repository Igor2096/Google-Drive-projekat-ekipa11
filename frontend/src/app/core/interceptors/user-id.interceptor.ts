import { HttpInterceptorFn } from '@angular/common/http';
import { environment } from '../../../environments/environment';

/**
 * PRIVREMENO: dodaje X-User-ID u svaki zahtev ka Files servisu.
 * Kada kolega zavrsi login, ovaj interceptor se menja tako da dodaje
 * "Authorization: Bearer <jwt>", a gateway iz tokena postavlja X-User-ID.
 */
export const userIdInterceptor: HttpInterceptorFn = (req, next) => {
  if (!req.url.startsWith(environment.filesApiUrl)) {
    return next(req);
  }
  return next(req.clone({ setHeaders: { 'X-User-ID': environment.devUserId } }));
};
