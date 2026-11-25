
import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';

import { environment } from '../../../environments/environment';
import { Notification } from '../models/notification.model';

@Injectable({
  providedIn: 'root'
})
export class NotificationService {

  private readonly http = inject(HttpClient);

  private readonly baseUrl = `${environment.apiUrl}/api/notifications/notifications`;

  getNotifications(userId: string): Observable<Notification[]> {
    return this.http.get<Notification[]>(this.baseUrl, {
      params: {
        user_id: userId
      }
    });
  }
}
