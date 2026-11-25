import { HttpInterceptorFn } from '@angular/common/http';

export const authInterceptor: HttpInterceptorFn = (req, next) => {
  const token = sessionStorage.getItem('accessToken');

  if (!token) {
    return next(req);
  }

  // Ne šaljemo token prilikom registracije, prijave i OTP verifikacije.
  const publicEndpoints = [
    '/register',
    '/login',
    '/verify-otp',
  ];

  const isPublicEndpoint = publicEndpoints.some((endpoint) =>
    req.url.endsWith(endpoint)
  );

  if (isPublicEndpoint) {
    return next(req);
  }

  const authRequest = req.clone({
    setHeaders: {
      Authorization: `Bearer ${token}`,
    },
  });

  return next(authRequest);
};