import { Component, inject } from '@angular/core';
import {
  AbstractControl,
  FormBuilder,
  ReactiveFormsModule,
  ValidationErrors,
  Validators,
} from '@angular/forms';
import { RouterLink } from '@angular/router';
import { finalize } from 'rxjs';
import { AuthService } from '../../../core/services/auth.service';

function passwordsMatch(control: AbstractControl): ValidationErrors | null {
  const password = control.get('new_password')?.value;
  const confirmation = control.get('confirm_password')?.value;
  return password === confirmation ? null : { passwordMismatch: true };
}

@Component({
  selector: 'app-change-password',
  standalone: true,
  imports: [ReactiveFormsModule, RouterLink],
  templateUrl: './change-password.html',
  styleUrl: './change-password.css',
})
export class ChangePassword {
  private readonly fb = inject(FormBuilder);
  private readonly authService = inject(AuthService);

  loading = false;
  errorMessage = '';
  successMessage = '';

  form = this.fb.nonNullable.group(
    {
      email: ['', [Validators.required, Validators.email]],
      old_password: ['', Validators.required],
      new_password: ['', [Validators.required, Validators.minLength(8)]],
      confirm_password: ['', Validators.required],
    },
    { validators: passwordsMatch },
  );

  submit(): void {
    this.errorMessage = '';
    this.successMessage = '';
    if (this.form.invalid) {
      this.form.markAllAsTouched();
      return;
    }
    const value = this.form.getRawValue();
    this.loading = true;
    this.authService
      .changePassword({
        email: value.email,
        old_password: value.old_password,
        new_password: value.new_password,
      })
      .pipe(finalize(() => (this.loading = false)))
      .subscribe({
        next: () => {
          this.successMessage = 'Lozinka je uspešno promenjena.';
          this.form.reset();
        },
        error: (error) => {
          this.errorMessage =
            error.error?.message || 'Promena lozinke nije uspela.';
        },
      });
  }
}
