
import { Component, inject, OnInit, signal } from '@angular/core';
import { DatePipe } from '@angular/common';

import { NotificationService } from '../../core/services/notification.service';
import { Notification } from '../../core/models/notification.model';

@Component({
  selector: 'app-notification-list',
  standalone: true,
  imports: [DatePipe],
  templateUrl: './notification-list.html',
    styleUrl: './notification-list.css'

})
export class NotificationList implements OnInit {

  private readonly notificationService = inject(NotificationService);

  readonly notifications = signal<Notification[]>([]);
  readonly loading = signal(true);
  readonly error = signal<string | null>(null);

  // Privremeni testni korisnik iz Cassandra baze
  private readonly userId = '11111111-1111-1111-1111-111111111111';

  ngOnInit(): void {
    this.loadNotifications();
  }

  private loadNotifications(): void {
    this.loading.set(true);
    this.error.set(null);

    this.notificationService.getNotifications(this.userId).subscribe({
      next: (data) => {
        this.notifications.set(data);
        this.loading.set(false);
      },
      error: (err) => {
        console.error('Greška pri učitavanju notifikacija:', err);
        this.error.set('Nije moguće učitati notifikacije.');
        this.loading.set(false);
      }
    });
  }
}
