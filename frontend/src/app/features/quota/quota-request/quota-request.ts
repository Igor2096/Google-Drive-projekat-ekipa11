
import { Component, inject, OnInit, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { QuotaInfo, QuotaService } from '../../../core/services/quota.service';

@Component({
  selector: 'app-quota-request',
  standalone: true,
  imports: [FormsModule],
  templateUrl: './quota-request.html',
  styleUrl: './quota-request.css',
})
export class QuotaRequest implements OnInit {
  private readonly quotaService = inject(QuotaService);

  protected requestedQuota = '';

  protected readonly quota = signal<QuotaInfo | null>(null);
  protected readonly quotaLoading = signal(true);
  protected readonly quotaError = signal('');

  protected readonly loading = signal(false);
  protected readonly successMessage = signal('');
  protected readonly errorMessage = signal('');

  ngOnInit(): void {
    this.loadQuota();
  }

  private loadQuota(): void {
    this.quotaLoading.set(true);
    this.quotaError.set('');

    this.quotaService.getMyQuota().subscribe({
      next: (data) => {
        this.quota.set(data);
        this.quotaLoading.set(false);
      },
      error: () => {
        this.quotaError.set('Nije moguće učitati podatke o kvoti.');
        this.quotaLoading.set(false);
      },
    });
  }

  protected formatGiB(bytes: number): string {
    return (bytes / (1024 * 1024 * 1024)).toFixed(2);
  }

  protected submitRequest(): void {
    const quotaInGiB = Number(this.requestedQuota);
    const currentQuota = this.quota();

    if (!currentQuota) {
      this.errorMessage.set('Podaci o trenutnoj kvoti nisu dostupni.');
      return;
    }

    if (!Number.isFinite(quotaInGiB) || quotaInGiB <= 0) {
      this.errorMessage.set('Unesite ispravnu kvotu veću od 0 GiB.');
      this.successMessage.set('');
      return;
    }

    if (quotaInGiB * 1024 * 1024 * 1024 <= currentQuota.storage_quota) {
      this.errorMessage.set(
        'Željena kvota mora biti veća od vaše trenutne kvote.',
      );
      this.successMessage.set('');
      return;
    }

    const quotaInBytes = Math.round(
      quotaInGiB * 1024 * 1024 * 1024,
    );

    this.loading.set(true);
    this.errorMessage.set('');
    this.successMessage.set('');

    this.quotaService.createRequest(quotaInBytes).subscribe({
      next: () => {
        this.successMessage.set(
          'Zahtev za povećanje kvote je uspešno poslat.',
        );
        this.requestedQuota = '';
        this.loading.set(false);
      },
      error: (error) => {
        this.errorMessage.set(
          error.error?.error ||
            'Došlo je do greške prilikom slanja zahteva.',
        );
        this.loading.set(false);
      },
    });
  }
}
