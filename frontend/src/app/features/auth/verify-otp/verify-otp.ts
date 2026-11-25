
import { Component, inject, ChangeDetectorRef } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { Router, RouterLink } from '@angular/router';
import { finalize } from 'rxjs';
import { AuthService } from '../../../core/services/auth.service';

@Component({
  selector: 'app-verify-otp',
  standalone: true,
  imports: [ReactiveFormsModule, RouterLink],
  templateUrl: './verify-otp.html',
  styleUrl: './verify-otp.css',
})
export class VerifyOtp {
  private readonly fb = inject(FormBuilder);
  private readonly authService = inject(AuthService);
  private readonly router = inject(Router);
  private readonly cdr = inject(ChangeDetectorRef);

  loading = false;
  errorMessage = '';

  otpForm = this.fb.nonNullable.group({
    otp: ['', [Validators.required, Validators.pattern(/^\d{6}$/)]],
  });

  verify(): void {
    this.errorMessage = '';

    const email = sessionStorage.getItem('pendingLoginEmail');

    if (!email) {
      this.errorMessage = 'Sesija prijave nije pronađena. Prijavite se ponovo.';
      return;
    }

    if (this.otpForm.invalid) {
      this.otpForm.markAllAsTouched();
      return;
    }

    this.loading = true;

    this.authService.verifyOtp({
      email,
      otp: this.otpForm.controls.otp.value,
    })
    .pipe(finalize(() => {
      this.loading = false;
      this.cdr.detectChanges();
    }))
    .subscribe({
      next: (response) => {
        sessionStorage.setItem('accessToken', response.access_token);
        sessionStorage.removeItem('pendingLoginEmail');

        this.router.navigate(['/']);
      },
      error: (error: any) => {
        this.errorMessage =
          error.error?.message || 'OTP kod nije ispravan ili je istekao.';
      },
    });
  }
}
