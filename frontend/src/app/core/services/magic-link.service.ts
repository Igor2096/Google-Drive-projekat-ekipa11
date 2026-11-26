import { HttpClient, HttpErrorResponse } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { environment } from '../../../environments/environment';
import { AuthResponse } from './auth.service';

/** Potvrda naloga (1.1) i prijava magicnim linkom (1.3). */
@Injectable({ providedIn: 'root' })
export class MagicLinkService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = environment.usersApiUrl;

  confirmEmail(token: string): Observable<unknown> {
    return this.http.get(`${this.baseUrl}/confirm-email`, { params: { token } });
  }

  requestMagicLink(email: string): Observable<unknown> {
    return this.http.post(`${this.baseUrl}/magic-link`, { email });
  }

  verifyMagicLink(token: string): Observable<AuthResponse> {
    return this.http.post<AuthResponse>(`${this.baseUrl}/magic-link/verify`, { token });
  }
}

/**
 * Users servis vraca greske kao obican tekst (http.Error), a ne kao JSON.
 * Ova funkcija vadi poruku u oba slucaja, da korisnik vidi pravi razlog
 * (npr. "Lozinka je istekla") umesto opste poruke.
 */
export function serverMessage(error: unknown, fallback: string): string {
  if (error instanceof HttpErrorResponse) {
    if (typeof error.error === 'string' && error.error.trim()) return error.error.trim();
    if (error.error?.message) return error.error.message;
    if (error.status === 0) return 'Server nije dostupan. Da li je prihvaćen sertifikat (https://localhost:8080/health)?';
  }
  return fallback;
}
