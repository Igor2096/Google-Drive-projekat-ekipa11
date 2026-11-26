
import { Component, inject, OnInit, signal } from '@angular/core';
import {
  AdminService,
  AdminUser,
  AdminQuotaRequest,
} from '../../../core/services/admin.service';

@Component({
  selector: 'app-admin-page',
  standalone: true,
  imports: [],
  templateUrl: './admin-page.html',
  styleUrl: './admin-page.css',
})
export class AdminPage implements OnInit {
  private readonly adminService = inject(AdminService);

  protected readonly users = signal<AdminUser[]>([]);
  protected readonly requests = signal<AdminQuotaRequest[]>([]);
  protected readonly loading = signal(true);
  protected readonly errorMessage = signal('');
  protected readonly successMessage = signal('');
  protected readonly processingId = signal('');

  ngOnInit(): void {
    this.loadData();
  }

  protected loadData(): void {
    this.loading.set(true);
    this.errorMessage.set('');

    this.adminService.getUsers().subscribe({
      next: (users) => this.users.set(users),
      error: () => this.errorMessage.set('Greška pri učitavanju korisnika.'),
    });

    this.adminService.getQuotaRequests().subscribe({
      next: (requests) => {
        this.requests.set(requests);
        this.loading.set(false);
      },
      error: () => {
        this.errorMessage.set('Greška pri učitavanju zahteva.');
        this.loading.set(false);
      },
    });
  }

  protected updateRequest(
    request: AdminQuotaRequest,
    status: 'approved' | 'rejected'
  ): void {
    this.processingId.set(request.id);
    this.errorMessage.set('');
    this.successMessage.set('');

    this.adminService.updateQuotaRequest(request.id, status).subscribe({
      next: () => {
        this.successMessage.set(
          status === 'approved'
            ? 'Zahtev je uspešno odobren.'
            : 'Zahtev je uspešno odbijen.'
        );
        this.processingId.set('');
        this.loadData();
      },
      error: (error) => {
        this.errorMessage.set(
          error.error?.error || 'Greška pri obradi zahteva.'
        );
        this.processingId.set('');
      },
    });
  }

  protected formatGiB(bytes: number): string {
    return (bytes / (1024 * 1024 * 1024)).toFixed(2);
  }

  protected formatDate(date: string): string {
    return new Date(date).toLocaleString('sr-RS');
  }
}
