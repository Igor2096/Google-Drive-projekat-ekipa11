import { HttpInterceptorFn } from '@angular/common/http';

export const userIdInterceptor: HttpInterceptorFn = (req, next) => {
  // Identitet korisnika sada određuje API Gateway na osnovu JWT tokena.
  // Frontend više ne šalje X-User-ID.
  return next(req);
};