import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';

import { environment } from '../../../environments/environment';
import { Notification } from '../models/notification.model';

@Injectable({
  providedIn: 'root',
})
export class NotificationService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = `${environment.notificationsApiUrl}/notifications`;

  /**
   * Notifikacije PRIJAVLJENOG korisnika. Ko je korisnik, odredjuje gateway
   * iz JWT tokena - klijent vise ne salje nikakav ID (zastita od citanja
   * tudjih notifikacija).
   *
   * Parametar je ostavljen samo radi kompatibilnosti sa starim pozivima i
   * namerno se ignorise.
   */
  getNotifications(_ignoredUserId?: string): Observable<Notification[]> {
    return this.http.get<Notification[]>(this.baseUrl);
  }
}
