import {
  FormBuilder,
  ReactiveFormsModule,
  Validators,
} from '@angular/forms';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { AuthService } from '../../../core/services/auth.service';
import { serverMessage } from '../../../core/services/magic-link.service';
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
  private readonly route = inject(ActivatedRoute);

  loading = false;
  errorMessage = '';
  readonly sessionExpired = this.route.snapshot.queryParamMap.get('sesija') === 'istekla';

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
        // Prikazuje se pravi razlog sa servera (nepotvrdjen nalog, istekla
        // lozinka...), a ne uvek ista opsta poruka.
        this.errorMessage = serverMessage(error, 'Neispravan email ili lozinka.');
      },
    });
}
}
