import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';

/**
 * Bez prijave se ne ulazi u Disk. Ovo je samo zgodnost za korisnika -
 * stvarnu zastitu radi gateway, koji proverava token na svakom zahtevu.
 */
export const authGuard: CanActivateFn = () => {
  const router = inject(Router);
  return sessionStorage.getItem('accessToken') ? true : router.createUrlTree(['/login']);
};
