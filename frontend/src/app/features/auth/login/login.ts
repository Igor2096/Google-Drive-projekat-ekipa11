import {
  FormBuilder,
  ReactiveFormsModule,
  Validators,
} from '@angular/forms';
import { Router, RouterLink } from '@angular/router';
import { AuthService } from '../../../core/services/auth.service';
import { finalize } from 'rxjs';
import { Component, inject, ChangeDetectorRef } from '@angular/core';

@Component({
  selector: 'app-login',
  standalone: true,
  imports: [ReactiveFormsModule, RouterLink],
  templateUrl: './login.html',
  styleUrl: './login.css',
})
export class Login {
  private readonly fb = inject(FormBuilder);
  private readonly authService = inject(AuthService);
  private readonly router = inject(Router);
  private readonly cdr = inject(ChangeDetectorRef); 

  loading = false;
  errorMessage = '';

  loginForm = this.fb.nonNullable.group({
    email: ['', [Validators.required, Validators.email]],
    password: ['', Validators.required],
  });

  onSubmit(): void {
  this.errorMessage = '';

  if (this.loginForm.invalid) {
    this.loginForm.markAllAsTouched();
    return;
  }

  this.loading = true;
  this.authService
    .login(this.loginForm.getRawValue())
    .pipe(
      finalize(() => {
         this.loading = false;
        this.cdr.detectChanges();

      }),
    )
    .subscribe({
      next: () => {
        sessionStorage.setItem(
          'pendingLoginEmail',
          this.loginForm.controls.email.value,
        );

        this.router.navigate(['/verify-otp']);
      },
      error: (error: any) => {
        this.errorMessage =
          error.error?.message || 'Neispravan email ili lozinka.';
      },
    });
}
}
