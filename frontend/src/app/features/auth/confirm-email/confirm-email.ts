import { Component, OnInit, inject, signal } from '@angular/core';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { MagicLinkService, serverMessage } from '../../../core/services/magic-link.service';

/** Otvara se iz mejla posle registracije: /confirm-email?token=... (1.1) */
@Component({
  selector: 'app-confirm-email',
  standalone: true,
  imports: [RouterLink],
  templateUrl: './confirm-email.html',
  styleUrl: '../login/login.css',
})
export class ConfirmEmail implements OnInit {
  private readonly route = inject(ActivatedRoute);
  private readonly api = inject(MagicLinkService);

  readonly state = signal<'loading' | 'ok' | 'error'>('loading');
  readonly message = signal('');

  ngOnInit(): void {
    const token = this.route.snapshot.queryParamMap.get('token');
    if (!token) {
      this.state.set('error');
      this.message.set('Link za potvrdu nije ispravan.');
      return;
    }
    this.api.confirmEmail(token).subscribe({
      next: () => this.state.set('ok'),
      error: (err) => {
        this.state.set('error');
        this.message.set(serverMessage(err, 'Potvrda naloga nije uspela.'));
      },
    });
  }
}
