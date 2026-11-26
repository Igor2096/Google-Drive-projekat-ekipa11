import { Component, OnInit, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { finalize } from 'rxjs';
import { MagicLinkService, serverMessage } from '../../../core/services/magic-link.service';

/**
 * Prijava magicnim linkom (1.3). Ista strana radi dve stvari:
 *  - bez tokena u URL-u: forma za unos mejla, na koji stize link
 *  - sa ?token=... (otvoreno iz mejla): token se menja za JWT i korisnik ulazi
 */
@Component({
  selector: 'app-magic-link',
  standalone: true,
  imports: [ReactiveFormsModule, RouterLink],
  templateUrl: './magic-link.html',
  styleUrl: '../login/login.css',
})
export class MagicLink implements OnInit {
  private readonly fb = inject(FormBuilder);
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly api = inject(MagicLinkService);

  readonly verifying = signal(false);
  readonly sending = signal(false);
  readonly sent = signal(false);
  readonly errorMessage = signal('');

  readonly form = this.fb.nonNullable.group({
    email: ['', [Validators.required, Validators.email, Validators.maxLength(254)]],
  });

  ngOnInit(): void {
    const token = this.route.snapshot.queryParamMap.get('token');
    if (token) {
      this.verify(token);
    }
  }

  send(): void {
    if (this.form.invalid) {
      this.form.markAllAsTouched();
      return;
    }
    this.errorMessage.set('');
    this.sending.set(true);
    this.api
      .requestMagicLink(this.form.controls.email.value.trim())
      .pipe(finalize(() => this.sending.set(false)))
      .subscribe({
        next: () => this.sent.set(true),
        error: (err) => this.errorMessage.set(serverMessage(err, 'Slanje linka nije uspelo.')),
      });
  }

  private verify(token: string): void {
    this.verifying.set(true);
    this.api
      .verifyMagicLink(token)
      .pipe(finalize(() => this.verifying.set(false)))
      .subscribe({
        next: (response) => {
          sessionStorage.setItem('accessToken', response.access_token);
          this.router.navigate(['/']);
        },
        error: (err) =>
          this.errorMessage.set(serverMessage(err, 'Link za prijavu nije ispravan ili je istekao.')),
      });
  }
}
